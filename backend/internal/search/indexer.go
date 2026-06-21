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
type Doc struct {
	CandidateID     string   `json:"candidate_id"`
	FullName        string   `json:"full_name"`
	Province        string   `json:"province"`
	Subregion       string   `json:"subregion"`
	AssignedStoreID *int     `json:"assigned_store_id"`
	Status          string   `json:"status"`
	AIScore         *float64 `json:"ai_score"`
}

// Indexer manages the candidate search index: schema creation, document upserts,
// and document deletes (PDPA erasure). The mock implementation is a no-op so
// local/CI need no Azure creds.
type Indexer interface {
	EnsureIndex(ctx context.Context) error
	UpsertBatch(ctx context.Context, docs []Doc) error
	Delete(ctx context.Context, candidateIDs []string) error
}

// NewIndexer selects the implementation by config — Azure push when
// AI_SEARCH_PROVIDER=azure, otherwise a no-op (mirrors NewSearcher).
func NewIndexer(cfg *config.Config) Indexer {
	if cfg.UsesAzureSearch() {
		return newAzureIndexer(cfg)
	}
	return noopIndexer{}
}

// noopIndexer is the mock-default: the Postgres trigram searcher needs no index,
// so every operation is a successful no-op.
type noopIndexer struct{}

func (noopIndexer) EnsureIndex(context.Context) error        { return nil }
func (noopIndexer) UpsertBatch(context.Context, []Doc) error { return nil }
func (noopIndexer) Delete(context.Context, []string) error   { return nil }

// azureIndexer pushes documents into an Azure AI Search index via the REST push
// API. Constructed only when AI_SEARCH_PROVIDER=azure.
type azureIndexer struct {
	endpoint string
	key      string
	index    string
	http     *http.Client
}

func newAzureIndexer(cfg *config.Config) *azureIndexer {
	return &azureIndexer{
		endpoint: strings.TrimRight(cfg.AzureSearchEndpoint, "/"),
		key:      cfg.AzureSearchKey,
		index:    cfg.AzureSearchIndex,
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
// surfaced.
func (s *azureIndexer) UpsertBatch(ctx context.Context, docs []Doc) error {
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
