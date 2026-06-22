package search

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/config"
)

func TestNewSearcher_DefaultsToPostgres(t *testing.T) {
	// Any non-"azure" value (incl. "mock" and empty string) selects Postgres.
	for _, provider := range []string{"mock", ""} {
		if _, ok := NewSearcher(&config.Config{AISearchProvider: provider}, nil, nil).(*pgSearcher); !ok {
			t.Fatalf("expected *pgSearcher for provider %q", provider)
		}
	}
}

func TestNewSearcher_AzureWhenConfigured(t *testing.T) {
	cfg := &config.Config{AISearchProvider: "azure", AzureSearchEndpoint: "https://x", AzureSearchKey: "k"}
	if _, ok := NewSearcher(cfg, nil, nil).(*azureSearcher); !ok {
		t.Fatal("expected *azureSearcher when AI_SEARCH_PROVIDER=azure")
	}
}

func TestQueryNormalize(t *testing.T) {
	q := Query{Page: 0, Limit: 0}
	q.normalize()
	if q.Page != 1 || q.Limit != DefaultLimit {
		t.Fatalf("defaults wrong: %+v", q)
	}
	q = Query{Limit: 9999}
	q.normalize()
	if q.Limit != MaxLimit {
		t.Fatalf("limit not clamped: %d", q.Limit)
	}
}

func TestScopeFilter(t *testing.T) {
	store := 7
	cases := []struct {
		name  string
		scope rbac.Scope
		want  string
	}{
		{"all", rbac.New("super_admin", nil, ""), ""},
		{"subregion", rbac.New("operation_director", nil, "East"), "subregion eq 'East'"},
		{"store", rbac.New("hr_staff", &store, ""), "assigned_store_id eq 7"},
		{"store-nil", rbac.New("hr_staff", nil, ""), "assigned_store_id eq -1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scopeFilter(tc.scope, Query{}); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestScopeFilter_EscapesQuotes(t *testing.T) {
	got := scopeFilter(rbac.New("operation_director", nil, "O'Hara"), Query{})
	if got != "subregion eq 'O''Hara'" {
		t.Fatalf("odata escaping wrong: %q", got)
	}
}

func TestAzureSearcher_MapsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"@odata.count":2,"value":[
			{"candidate_id":"c1","full_name":"สมชาย","province":"กรุงเทพ","status":"scored","ai_score":88},
			{"candidate_id":"c2","full_name":"สมหญิง","province":"นนทบุรี","status":"hired","ai_score":null}]}`))
	}))
	defer srv.Close()

	s := &azureSearcher{endpoint: srv.URL, key: "k", index: "candidates", http: &http.Client{Timeout: 5 * time.Second}}
	hits, total, err := s.Search(context.Background(), Query{Text: "cashier"}, rbac.New("super_admin", nil, ""))
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(hits) != 2 {
		t.Fatalf("expected 2 hits/total, got hits=%d total=%d", len(hits), total)
	}
	if hits[0].CandidateID != "c1" || hits[0].AIScore == nil || *hits[0].AIScore != 88 {
		t.Errorf("hit0 mapped wrong: %+v", hits[0])
	}
	if hits[1].AIScore != nil {
		t.Errorf("hit1 score should be nil, got %v", *hits[1].AIScore)
	}
}

// captureSearchBody runs one Search against a test server and returns the request
// body the searcher posted, so tests can assert the hybrid vector query shape.
func captureSearchBody(t *testing.T, s *azureSearcher, q Query) map[string]any {
	t.Helper()
	return captureSearchBodyScoped(t, s, q, rbac.New("super_admin", nil, ""))
}

func captureSearchBodyScoped(t *testing.T, s *azureSearcher, q Query, scope rbac.Scope) map[string]any {
	t.Helper()
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"@odata.count":0,"value":[]}`)
	}))
	defer srv.Close()
	s.endpoint = srv.URL
	s.http = &http.Client{Timeout: 5 * time.Second}
	if _, _, err := s.Search(context.Background(), q, scope); err != nil {
		t.Fatalf("Search: %v", err)
	}
	return body
}

// TestAzureSearch_HybridAddsVectorQuery asserts that, with an embedder and query
// text, the request carries a vectorQueries element (kind/vector/fields/k) matching
// the Azure 2024-07-01 RawVectorQuery shape.
func TestAzureSearch_HybridAddsVectorQuery(t *testing.T) {
	s := &azureSearcher{key: "k", index: "candidates", embedder: fakeEmbedder{dims: 4}}
	body := captureSearchBody(t, s, Query{Text: "ช่างไฟ", Limit: 20, Page: 1})

	vqs, ok := body["vectorQueries"].([]any)
	if !ok || len(vqs) != 1 {
		t.Fatalf("expected 1 vectorQuery, got %v", body["vectorQueries"])
	}
	vq := vqs[0].(map[string]any)
	if vq["kind"] != "vector" {
		t.Errorf("kind = %v, want vector", vq["kind"])
	}
	if vq["fields"] != "content_vector" {
		t.Errorf("fields = %v, want content_vector", vq["fields"])
	}
	if vec, ok := vq["vector"].([]any); !ok || len(vec) != 4 {
		t.Errorf("vector = %v, want 4-element array", vq["vector"])
	}
	if vq["k"] == nil {
		t.Error("k missing from vector query")
	}
	if body["vectorFilterMode"] != "preFilter" {
		t.Errorf("vectorFilterMode = %v, want preFilter", body["vectorFilterMode"])
	}
}

// TestAzureSearch_ScopedHybridKeepsFilter proves the RBAC scope filter survives in
// hybrid mode and runs as a preFilter, so a store-scoped user's vector recall is
// drawn from in-scope candidates rather than a company-wide top-k.
func TestAzureSearch_ScopedHybridKeepsFilter(t *testing.T) {
	store := 7
	s := &azureSearcher{key: "k", index: "candidates", embedder: fakeEmbedder{dims: 4}}
	body := captureSearchBodyScoped(t, s, Query{Text: "ช่างไฟ", Limit: 20, Page: 1},
		rbac.New("hr_staff", &store, ""))

	if _, present := body["vectorQueries"]; !present {
		t.Fatal("scoped hybrid search missing vectorQueries")
	}
	if body["filter"] != "assigned_store_id eq 7" {
		t.Errorf("filter = %v, want store scope clause", body["filter"])
	}
	if body["vectorFilterMode"] != "preFilter" {
		t.Errorf("vectorFilterMode = %v, want preFilter (else scoped recall is gutted)", body["vectorFilterMode"])
	}
}

// TestAzureSearch_NoEmbedderKeywordOnly confirms the request stays keyword-only
// when no embedder is wired.
func TestAzureSearch_NoEmbedderKeywordOnly(t *testing.T) {
	s := &azureSearcher{key: "k", index: "candidates"} // embedder nil
	body := captureSearchBody(t, s, Query{Text: "ช่างไฟ", Limit: 20, Page: 1})
	if _, present := body["vectorQueries"]; present {
		t.Errorf("keyword-only search must omit vectorQueries, got %v", body["vectorQueries"])
	}
}

// TestAzureSearch_EmbedFailureFallsBack confirms a failing embedder degrades to a
// keyword-only query without erroring the search.
func TestAzureSearch_EmbedFailureFallsBack(t *testing.T) {
	s := &azureSearcher{key: "k", index: "candidates", embedder: fakeEmbedder{err: errEmbed}}
	body := captureSearchBody(t, s, Query{Text: "ช่างไฟ", Limit: 20, Page: 1})
	if _, present := body["vectorQueries"]; present {
		t.Errorf("embed failure should fall back to keyword-only, got %v", body["vectorQueries"])
	}
	if body["search"] != "ช่างไฟ" {
		t.Errorf("search text should still be present, got %v", body["search"])
	}
}
