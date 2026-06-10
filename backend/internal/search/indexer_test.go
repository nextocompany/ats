package search

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestCandidateIndexSchema(t *testing.T) {
	raw, err := json.Marshal(candidateIndexSchema("candidates"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(raw)
	for _, want := range []string{
		`"name":"candidates"`,
		`"name":"candidate_id"`, `"key":true`,
		`"name":"full_name"`, `"analyzer":"th.microsoft"`,
		`"name":"assigned_store_id"`, `"type":"Edm.Int32"`,
		`"name":"ai_score"`, `"type":"Edm.Double"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("schema missing %q\nin: %s", want, s)
		}
	}
}

// newTestIndexer points an azureIndexer at a test server.
func newTestIndexer(url string) *azureIndexer {
	return &azureIndexer{endpoint: url, key: "k", index: "candidates", http: http.DefaultClient}
}

func TestUpsertBatch_ChunksOverMax(t *testing.T) {
	var mu sync.Mutex
	var requests, docsSeen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Value []map[string]any `json:"value"`
		}
		_ = json.Unmarshal(body, &payload)
		mu.Lock()
		requests++
		docsSeen += len(payload.Value)
		mu.Unlock()
		// Echo a per-doc success status.
		resp := indexResponse{}
		for _, d := range payload.Value {
			resp.Value = append(resp.Value, struct {
				Key          string `json:"key"`
				Status       bool   `json:"status"`
				ErrorMessage string `json:"errorMessage"`
				StatusCode   int    `json:"statusCode"`
			}{Key: d["candidate_id"].(string), Status: true, StatusCode: 200})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	idx := newTestIndexer(srv.URL)
	docs := make([]Doc, 1200)
	for i := range docs {
		docs[i] = Doc{CandidateID: idFromInt(i)}
	}
	if err := idx.UpsertBatch(context.Background(), docs); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	// 1200 docs / 500 per batch = 3 requests (500 + 500 + 200).
	if requests != 3 {
		t.Errorf("requests = %d, want 3", requests)
	}
	if docsSeen != 1200 {
		t.Errorf("docsSeen = %d, want 1200", docsSeen)
	}
}

func TestUpsertBatch_PartialFailureSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 200 OK overall, but one doc failed — must be surfaced as an error.
		_, _ = io.WriteString(w, `{"value":[{"key":"a","status":true,"statusCode":200},{"key":"b","status":false,"statusCode":400,"errorMessage":"bad field"}]}`)
	}))
	defer srv.Close()

	idx := newTestIndexer(srv.URL)
	err := idx.UpsertBatch(context.Background(), []Doc{{CandidateID: "a"}, {CandidateID: "b"}})
	if err == nil || !strings.Contains(err.Error(), "bad field") {
		t.Fatalf("want error surfacing doc failure, got %v", err)
	}
}

func TestUpsertBatch_HardStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":{"message":"query key cannot index"}}`)
	}))
	defer srv.Close()
	idx := newTestIndexer(srv.URL)
	err := idx.UpsertBatch(context.Background(), []Doc{{CandidateID: "a"}})
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("want 403 error, got %v", err)
	}
}

func TestNoopIndexer_NoCalls(t *testing.T) {
	var n noopIndexer
	if err := n.EnsureIndex(context.Background()); err != nil {
		t.Errorf("EnsureIndex: %v", err)
	}
	if err := n.UpsertBatch(context.Background(), []Doc{{CandidateID: "x"}}); err != nil {
		t.Errorf("UpsertBatch: %v", err)
	}
}

// idFromInt makes a stable distinct id string per index without Math/rand.
func idFromInt(i int) string {
	const hex = "0123456789abcdef"
	b := []byte("00000000")
	for j := len(b) - 1; i > 0 && j >= 0; j-- {
		b[j] = hex[i&0xf]
		i >>= 4
	}
	return string(b)
}
