package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nexto/hr-ats/pkg/config"
)

// maxBatch caps documents per /docs/index request. Azure allows ≤1000 docs /
// 16MB per batch; 500 stays well under both for our small per-doc payload.
const maxBatch = 500

// Doc is one candidate's searchable projection — their best application's
// status/score/store plus the candidate's name/province/subregion. JSON keys
// match the index field names (and azure.go's query response) exactly.
//
// Content / ContentVector power semantic search. Content is the text blob
// (name + province + ai_summary) the projection always builds; ContentVector is
// filled at index time by the embedder. Both are omitempty so a keyword-only
// index (semantic off) never receives fields it doesn't define: the indexer
// also clears Content in that mode (see pushChunk).
type Doc struct {
	CandidateID     string    `json:"candidate_id"`
	FullName        string    `json:"full_name"`
	Province        string    `json:"province"`
	Subregion       string    `json:"subregion"`
	AssignedStoreID *int      `json:"assigned_store_id"`
	Status          string    `json:"status"`
	AIScore         *float64  `json:"ai_score"`
	Content         string    `json:"content,omitempty"`
	ContentVector   []float32 `json:"content_vector,omitempty"`
}

// Indexer manages the candidate search index: schema creation, document upserts,
// document deletes (PDPA erasure), and index drop (semantic migration). The mock
// implementation is a no-op so local/CI need no Azure creds.
type Indexer interface {
	EnsureIndex(ctx context.Context) error
	DropIndex(ctx context.Context) error
	UpsertBatch(ctx context.Context, docs []Doc) error
	Delete(ctx context.Context, candidateIDs []string) error
}

// NewIndexer selects the implementation by config — Azure push when
// AI_SEARCH_PROVIDER=azure, otherwise a no-op (mirrors NewSearcher). A non-nil
// embedder enables semantic indexing (content + content_vector); nil keeps the
// index keyword-only. Delete-only callers (PDPA erasure) may pass nil.
func NewIndexer(cfg *config.Config, embedder Embedder) Indexer {
	if cfg.UsesAzureSearch() {
		return newAzureIndexer(cfg, embedder)
	}
	return noopIndexer{}
}

// noopIndexer is the mock-default: the Postgres trigram searcher needs no index,
// so every operation is a successful no-op.
type noopIndexer struct{}

func (noopIndexer) EnsureIndex(context.Context) error        { return nil }
func (noopIndexer) DropIndex(context.Context) error          { return nil }
func (noopIndexer) UpsertBatch(context.Context, []Doc) error { return nil }
func (noopIndexer) Delete(context.Context, []string) error   { return nil }

// azureIndexer pushes documents into an Azure AI Search index via the REST push
// API. Constructed only when AI_SEARCH_PROVIDER=azure. A non-nil embedder turns
// on semantic indexing; dims must match the embedder's output and the index
// schema's vector field.
type azureIndexer struct {
	endpoint string
	key      string
	index    string
	dims     int
	embedder Embedder
	semantic bool
	http     *http.Client
}

func newAzureIndexer(cfg *config.Config, embedder Embedder) *azureIndexer {
	return &azureIndexer{
		endpoint: strings.TrimRight(cfg.AzureSearchEndpoint, "/"),
		key:      cfg.AzureSearchKey,
		index:    cfg.AzureSearchIndex,
		dims:     cfg.AzureOpenAIEmbedDims,
		embedder: embedder,
		semantic: embedder != nil,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// indexAction wraps a Doc with the Azure document action. mergeOrUpload behaves
// like upload for new keys and merge for existing — an idempotent upsert.
type indexAction struct {
	Action string `json:"@search.action"`
	Doc
}

// deleteAction removes a document by key only. Azure's "delete" action needs just
// the key field (candidate_id); sending the rest would be ignored, so it is omitted.
type deleteAction struct {
	Action      string `json:"@search.action"`
	CandidateID string `json:"candidate_id"`
}

type indexResponse struct {
	Value []struct {
		Key          string `json:"key"`
		Status       bool   `json:"status"`
		ErrorMessage string `json:"errorMessage"`
		StatusCode   int    `json:"statusCode"`
	} `json:"value"`
}

// UpsertBatch upserts docs in chunks of maxBatch. /docs/index returns 200 even
// on per-document failures, so each response is inspected and the first failure
// surfaced. When semantic is on, every doc's Content is embedded first; an embed
// failure fails the whole batch rather than pushing vector-less docs (which would
// be silently invisible to vector search).
func (s *azureIndexer) UpsertBatch(ctx context.Context, docs []Doc) error {
	if s.semantic {
		if err := s.embedDocs(ctx, docs); err != nil {
			return err
		}
	}
	for start := 0; start < len(docs); start += maxBatch {
		end := start + maxBatch
		if end > len(docs) {
			end = len(docs)
		}
		if err := s.pushChunk(ctx, docs[start:end]); err != nil {
			return err
		}
	}
	return nil
}

// embedDocs fills each doc's ContentVector from its Content. Mutates the docs in
// place (caller owns the slice). The embedder handles its own sub-batching/retry.
func (s *azureIndexer) embedDocs(ctx context.Context, docs []Doc) error {
	if len(docs) == 0 {
		return nil
	}
	texts := make([]string, len(docs))
	for i := range docs {
		texts[i] = docs[i].Content
	}
	vecs, err := s.embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("search: embed docs: %w", err)
	}
	if len(vecs) != len(docs) {
		return fmt.Errorf("search: embed returned %d vectors for %d docs", len(vecs), len(docs))
	}
	for i := range docs {
		docs[i].ContentVector = vecs[i]
	}
	return nil
}

// Delete removes candidate documents from the index by key, in chunks of
// maxBatch. Used by PDPA erasure so a forgotten subject leaves no searchable
// trace. A no-op for an empty list.
func (s *azureIndexer) Delete(ctx context.Context, candidateIDs []string) error {
	for start := 0; start < len(candidateIDs); start += maxBatch {
		end := start + maxBatch
		if end > len(candidateIDs) {
			end = len(candidateIDs)
		}
		actions := make([]deleteAction, 0, end-start)
		for _, id := range candidateIDs[start:end] {
			actions = append(actions, deleteAction{Action: "delete", CandidateID: id})
		}
		if err := s.postActions(ctx, actions); err != nil {
			return err
		}
	}
	return nil
}

func (s *azureIndexer) pushChunk(ctx context.Context, docs []Doc) error {
	actions := make([]indexAction, 0, len(docs))
	for _, d := range docs {
		if !s.semantic {
			// Keyword-only index has no content/content_vector fields; clearing
			// them lets omitempty drop them so Azure doesn't reject unknown fields.
			d.Content = ""
			d.ContentVector = nil
		}
		actions = append(actions, indexAction{Action: "mergeOrUpload", Doc: d})
	}
	return s.postActions(ctx, actions)
}

// postActions marshals an action batch and POSTs it to /docs/index, surfacing the
// first per-document failure. Shared by upsert and delete (both are batched
// document actions on the same endpoint).
func (s *azureIndexer) postActions(ctx context.Context, actions any) error {
	body, err := json.Marshal(map[string]any{"value": actions})
	if err != nil {
		return fmt.Errorf("search: index marshal: %w", err)
	}

	url := fmt.Sprintf("%s/indexes/%s/docs/index?api-version=%s", s.endpoint, s.index, searchAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("search: index request: %w", err)
	}
	req.Header.Set("api-key", s.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("search: index push: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	// 200 = processed (check per-doc status); 207 = partial. Anything else is a
	// hard failure (auth, bad index, throttling).
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMultiStatus {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("search: index push status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var ir indexResponse
	if err := json.NewDecoder(resp.Body).Decode(&ir); err != nil {
		return fmt.Errorf("search: index decode: %w", err)
	}
	for _, r := range ir.Value {
		if !r.Status {
			return fmt.Errorf("search: index doc %s failed (%d): %s", r.Key, r.StatusCode, r.ErrorMessage)
		}
	}
	return nil
}
