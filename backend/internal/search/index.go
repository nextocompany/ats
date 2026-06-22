package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// vectorProfileName / vectorAlgoName name the HNSW vector-search config blocks.
const (
	vectorProfileName = "candidate-hnsw-profile"
	vectorAlgoName    = "candidate-hnsw"
)

// candidateIndexSchema defines the Azure AI Search index the query side
// (azure.go) reads. Field names MUST match azureSearchResponse + scopeFilter:
// candidate_id (key), full_name/province (searchable, Thai analyzer), subregion
// + assigned_store_id + status + ai_score (filterable for the RBAC/score filter).
// Thai names have no word spaces → th.microsoft tokenizes them correctly where
// the default (Lucene) analyzer would treat a full name as one opaque token.
//
// When semantic is true the schema also carries a `content` text blob and a
// `content_vector` field (Collection(Edm.Single), dims) wired to an HNSW
// vectorSearch profile, enabling hybrid keyword+vector ranking. dims MUST match
// the embedder's output (config AZURE_OPENAI_EMBED_DIMS) or vector pushes fail.
func candidateIndexSchema(name string, dims int, semantic bool) map[string]any {
	field := func(n, t string, opts map[string]any) map[string]any {
		f := map[string]any{"name": n, "type": t}
		for k, v := range opts {
			f[k] = v
		}
		return f
	}
	fields := []map[string]any{
		field("candidate_id", "Edm.String", map[string]any{"key": true, "retrievable": true}),
		field("full_name", "Edm.String", map[string]any{"searchable": true, "retrievable": true, "analyzer": "th.microsoft"}),
		field("province", "Edm.String", map[string]any{"searchable": true, "retrievable": true, "analyzer": "th.microsoft"}),
		field("subregion", "Edm.String", map[string]any{"filterable": true, "retrievable": true}),
		field("assigned_store_id", "Edm.Int32", map[string]any{"filterable": true, "retrievable": true}),
		field("status", "Edm.String", map[string]any{"filterable": true, "retrievable": true}),
		field("ai_score", "Edm.Double", map[string]any{"filterable": true, "sortable": true, "retrievable": true}),
	}
	schema := map[string]any{"name": name}
	if semantic {
		fields = append(fields,
			field("content", "Edm.String", map[string]any{"searchable": true, "retrievable": false, "analyzer": "th.microsoft"}),
			field("content_vector", "Collection(Edm.Single)", map[string]any{
				"searchable": true, "retrievable": false,
				"dimensions": dims, "vectorSearchProfile": vectorProfileName,
			}),
		)
		schema["vectorSearch"] = map[string]any{
			"algorithms": []map[string]any{{
				"name": vectorAlgoName, "kind": "hnsw",
				"hnswParameters": map[string]any{
					"metric": "cosine", "m": 4, "efConstruction": 400, "efSearch": 500,
				},
			}},
			"profiles": []map[string]any{{
				"name": vectorProfileName, "algorithm": vectorAlgoName,
			}},
		}
	}
	schema["fields"] = fields
	return schema
}

// EnsureIndex creates-or-updates the candidates index (idempotent). Requires an
// ADMIN api-key (a query-only key returns 403). Safe to call at startup. Note:
// PUT cannot ADD a vector field to a pre-existing keyword-only index: Azure
// rejects that as an incompatible update. The semantic migration therefore goes
// DropIndex → EnsureIndex (see cmd/reindex -recreate); at app startup this PUT
// is a no-op against an already-correct index.
func (s *azureIndexer) EnsureIndex(ctx context.Context) error {
	body, err := json.Marshal(candidateIndexSchema(s.index, s.dims, s.semantic))
	if err != nil {
		return fmt.Errorf("search: index schema marshal: %w", err)
	}
	url := fmt.Sprintf("%s/indexes/%s?api-version=%s", s.endpoint, s.index, searchAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("search: index request: %w", err)
	}
	req.Header.Set("api-key", s.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("search: ensure index: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	// Create-or-update returns 201 (created) or 200/204 (updated).
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("search: ensure index status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// DropIndex deletes the candidates index. Used by the reindex migration to swap a
// keyword-only index for a vector-capable one (PUT can't add a vector field in
// place). A 404 is treated as success: the index is already gone. Requires an
// ADMIN api-key.
func (s *azureIndexer) DropIndex(ctx context.Context) error {
	url := fmt.Sprintf("%s/indexes/%s?api-version=%s", s.endpoint, s.index, searchAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("search: drop index request: %w", err)
	}
	req.Header.Set("api-key", s.key)

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("search: drop index: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("search: drop index status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
