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

	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/config"
)

const searchAPIVersion = "2024-07-01"

// azureSearcher queries an Azure AI Search index. Index population (pushing
// candidates → index) is an ops/ingestion concern and out of scope; this is
// query-only. Constructed only when AI_SEARCH_PROVIDER=azure, so missing
// credentials never affect the mock/CI path.
type azureSearcher struct {
	endpoint string
	key      string
	index    string
	http     *http.Client
}

func newAzureSearcher(cfg *config.Config) *azureSearcher {
	return &azureSearcher{
		endpoint: strings.TrimRight(cfg.AzureSearchEndpoint, "/"),
		key:      cfg.AzureSearchKey,
		index:    cfg.AzureSearchIndex,
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

type azureSearchRequest struct {
	Search string `json:"search"`
	Filter string `json:"filter,omitempty"`
	Top    int    `json:"top"`
	Skip   int    `json:"skip"`
	Count  bool   `json:"count"`
}

type azureSearchResponse struct {
	Count int `json:"@odata.count"`
	Value []struct {
		CandidateID string   `json:"candidate_id"`
		FullName    string   `json:"full_name"`
		Province    string   `json:"province"`
		Status      string   `json:"status"`
		AIScore     *float64 `json:"ai_score"`
	} `json:"value"`
}

func (s *azureSearcher) Search(ctx context.Context, q Query, scope rbac.Scope) ([]Hit, int, error) {
	q.normalize()

	reqBody := azureSearchRequest{
		Search: orAll(q.Text),
		Filter: scopeFilter(scope, q),
		Top:    q.Limit,
		Skip:   (q.Page - 1) * q.Limit,
		Count:  true,
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("search: azure marshal: %w", err)
	}

	url := fmt.Sprintf("%s/indexes/%s/docs/search?api-version=%s", s.endpoint, s.index, searchAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, fmt.Errorf("search: azure request: %w", err)
	}
	req.Header.Set("api-key", s.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("search: azure call: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		// Surface a (truncated) body — Azure Search errors carry actionable detail.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, 0, fmt.Errorf("search: azure status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var sr azureSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, 0, fmt.Errorf("search: azure decode: %w", err)
	}
	hits := make([]Hit, 0, len(sr.Value))
	for _, v := range sr.Value {
		hits = append(hits, Hit{
			CandidateID: v.CandidateID, FullName: v.FullName, Province: v.Province,
			Status: v.Status, AIScore: v.AIScore,
		})
	}
	return hits, sr.Count, nil
}

// orAll returns "*" (match-all) for an empty query so a blank text still paginates.
func orAll(text string) string {
	if strings.TrimSpace(text) == "" {
		return "*"
	}
	return text
}

// scopeFilter pushes the RBAC scope to the index as an OData filter so the index
// never returns out-of-scope candidates. Requires the index to carry `subregion`
// and `assigned_store_id` fields.
func scopeFilter(scope rbac.Scope, q Query) string {
	var clauses []string
	switch scope.Kind() {
	case rbac.KindSubregion:
		clauses = append(clauses, fmt.Sprintf("subregion eq '%s'", escapeOData(scope.Subregion)))
	case rbac.KindStore:
		if scope.StoreID == nil {
			clauses = append(clauses, "assigned_store_id eq -1") // scoped user without a store sees nothing
		} else {
			clauses = append(clauses, fmt.Sprintf("assigned_store_id eq %d", *scope.StoreID))
		}
	}
	if q.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status eq '%s'", escapeOData(q.Status)))
	}
	if q.MinScore != nil {
		clauses = append(clauses, fmt.Sprintf("ai_score ge %g", *q.MinScore))
	}
	return strings.Join(clauses, " and ")
}

// escapeOData escapes single quotes per the OData string-literal rule.
func escapeOData(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
