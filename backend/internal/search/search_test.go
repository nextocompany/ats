package search

import (
	"context"
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
		if _, ok := NewSearcher(&config.Config{AISearchProvider: provider}, nil).(*pgSearcher); !ok {
			t.Fatalf("expected *pgSearcher for provider %q", provider)
		}
	}
}

func TestNewSearcher_AzureWhenConfigured(t *testing.T) {
	cfg := &config.Config{AISearchProvider: "azure", AzureSearchEndpoint: "https://x", AzureSearchKey: "k"}
	if _, ok := NewSearcher(cfg, nil).(*azureSearcher); !ok {
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
