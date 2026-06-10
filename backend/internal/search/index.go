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

// candidateIndexSchema defines the Azure AI Search index the query side
// (azure.go) reads. Field names MUST match azureSearchResponse + scopeFilter:
// candidate_id (key), full_name/province (searchable, Thai analyzer), subregion
// + assigned_store_id + status + ai_score (filterable for the RBAC/score filter).
// Thai names have no word spaces → th.microsoft tokenizes them correctly where
// the default (Lucene) analyzer would treat a full name as one opaque token.
func candidateIndexSchema(name string) map[string]any {
	field := func(n, t string, opts map[string]any) map[string]any {
		f := map[string]any{"name": n, "type": t}
		for k, v := range opts {
			f[k] = v
		}
		return f
	}
	return map[string]any{
		"name": name,
		"fields": []map[string]any{
			field("candidate_id", "Edm.String", map[string]any{"key": true, "retrievable": true}),
			field("full_name", "Edm.String", map[string]any{"searchable": true, "retrievable": true, "analyzer": "th.microsoft"}),
			field("province", "Edm.String", map[string]any{"searchable": true, "retrievable": true, "analyzer": "th.microsoft"}),
			field("subregion", "Edm.String", map[string]any{"filterable": true, "retrievable": true}),
			field("assigned_store_id", "Edm.Int32", map[string]any{"filterable": true, "retrievable": true}),
			field("status", "Edm.String", map[string]any{"filterable": true, "retrievable": true}),
			field("ai_score", "Edm.Double", map[string]any{"filterable": true, "sortable": true, "retrievable": true}),
		},
	}
}

// EnsureIndex creates-or-updates the candidates index (idempotent). Requires an
// ADMIN api-key (a query-only key returns 403). Safe to call at startup.
func (s *azureIndexer) EnsureIndex(ctx context.Context) error {
	body, err := json.Marshal(candidateIndexSchema(s.index))
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
