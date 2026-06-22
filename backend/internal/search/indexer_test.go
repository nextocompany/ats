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
	raw, err := json.Marshal(candidateIndexSchema("candidates", 1536, false))
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
	// Keyword-only schema must carry NO vector field or vectorSearch block.
	for _, absent := range []string{`content_vector`, `vectorSearch`, `vectorSearchProfile`} {
		if strings.Contains(s, absent) {
			t.Errorf("keyword schema should not contain %q\nin: %s", absent, s)
		}
	}
}

// TestCandidateIndexSchema_Semantic asserts the vector field + HNSW vectorSearch
// block appear with the configured dims when semantic is on. Field/property names
// match Azure AI Search api-version 2024-07-01 (k, vectorSearchProfile,
// algorithms+profiles), see azure.go vectorQuery for the query side.
func TestCandidateIndexSchema_Semantic(t *testing.T) {
	raw, err := json.Marshal(candidateIndexSchema("candidates", 1536, true))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(raw)
	for _, want := range []string{
		`"name":"content"`,
		`"name":"content_vector"`, `"type":"Collection(Edm.Single)"`,
		`"dimensions":1536`, `"vectorSearchProfile":"candidate-hnsw-profile"`,
		`"vectorSearch":`, `"algorithms":`, `"profiles":`,
		`"kind":"hnsw"`, `"metric":"cosine"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("semantic schema missing %q\nin: %s", want, s)
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
	if err := n.Delete(context.Background(), []string{"x"}); err != nil {
		t.Errorf("Delete: %v", err)
	}
}

// TestDelete_BuildsKeyOnlyActions asserts the PDPA erasure delete posts one
// "delete" action per candidate carrying ONLY the candidate_id key - Azure
// ignores non-key fields on delete, and leaking the rest would be needless PII.
func TestDelete_BuildsKeyOnlyActions(t *testing.T) {
	var got struct {
		Value []map[string]any `json:"value"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = io.WriteString(w, `{"value":[{"key":"c1","status":true,"statusCode":200},{"key":"c2","status":true,"statusCode":200}]}`)
	}))
	defer srv.Close()

	idx := newTestIndexer(srv.URL)
	if err := idx.Delete(context.Background(), []string{"c1", "c2"}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(got.Value) != 2 {
		t.Fatalf("expected 2 delete actions, got %d", len(got.Value))
	}
	for i, want := range []string{"c1", "c2"} {
		a := got.Value[i]
		if a["@search.action"] != "delete" {
			t.Errorf("action %d: want @search.action=delete, got %v", i, a["@search.action"])
		}
		if a["candidate_id"] != want {
			t.Errorf("action %d: want candidate_id=%s, got %v", i, want, a["candidate_id"])
		}
		if len(a) != 2 { // action + key only
			t.Errorf("action %d: want key-only payload (2 fields), got %d: %v", i, len(a), a)
		}
	}
}

// TestDelete_ChunksOverMax confirms deletes batch at maxBatch like upserts.
func TestDelete_ChunksOverMax(t *testing.T) {
	var mu sync.Mutex
	var requests, keysSeen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Value []map[string]any `json:"value"`
		}
		_ = json.Unmarshal(body, &payload)
		mu.Lock()
		requests++
		keysSeen += len(payload.Value)
		mu.Unlock()
		var b strings.Builder
		b.WriteString(`{"value":[`)
		for i := range payload.Value {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(`{"key":"k","status":true,"statusCode":200}`)
		}
		b.WriteString(`]}`)
		_, _ = io.WriteString(w, b.String())
	}))
	defer srv.Close()

	idx := newTestIndexer(srv.URL)
	ids := make([]string, 1200)
	for i := range ids {
		ids[i] = idFromInt(i + 1)
	}
	if err := idx.Delete(context.Background(), ids); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if requests != 3 { // 500 + 500 + 200
		t.Errorf("requests = %d, want 3", requests)
	}
	if keysSeen != 1200 {
		t.Errorf("keysSeen = %d, want 1200", keysSeen)
	}
}

// TestDelete_EmptyIsNoop ensures deleting nothing makes no request.
func TestDelete_EmptyIsNoop(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = io.WriteString(w, `{"value":[]}`)
	}))
	defer srv.Close()

	idx := newTestIndexer(srv.URL)
	if err := idx.Delete(context.Background(), nil); err != nil {
		t.Fatalf("Delete empty: %v", err)
	}
	if called {
		t.Error("expected no HTTP request for empty delete list")
	}
}

// fakeEmbedder returns fixed-length vectors (or an error) for the index/query
// embed-path tests, with no network call.
type fakeEmbedder struct {
	dims int
	err  error
}

func (f fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		v := make([]float32, f.dims)
		for j := range v {
			v[j] = float32(i + 1) // distinct, non-zero per doc
		}
		out[i] = v
	}
	return out, nil
}

// TestUpsertBatch_SemanticEmbedsAndSendsVector verifies that with an embedder the
// indexer embeds each doc's Content and posts content + content_vector.
func TestUpsertBatch_SemanticEmbedsAndSendsVector(t *testing.T) {
	var got struct {
		Value []map[string]any `json:"value"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_, _ = io.WriteString(w, `{"value":[{"key":"c1","status":true,"statusCode":200}]}`)
	}))
	defer srv.Close()

	idx := &azureIndexer{endpoint: srv.URL, key: "k", index: "candidates",
		dims: 8, embedder: fakeEmbedder{dims: 8}, semantic: true, http: http.DefaultClient}
	err := idx.UpsertBatch(context.Background(), []Doc{{CandidateID: "c1", FullName: "สมชาย", Content: "สมชาย กรุงเทพ"}})
	if err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	if len(got.Value) != 1 {
		t.Fatalf("expected 1 action, got %d", len(got.Value))
	}
	a := got.Value[0]
	if a["content"] != "สมชาย กรุงเทพ" {
		t.Errorf("content = %v, want the blob", a["content"])
	}
	vec, ok := a["content_vector"].([]any)
	if !ok || len(vec) != 8 {
		t.Errorf("content_vector = %v, want 8-element array", a["content_vector"])
	}
}

// TestUpsertBatch_KeywordStripsContent ensures a keyword-only indexer (no
// embedder) never sends content/content_vector: the index has no such fields.
func TestUpsertBatch_KeywordStripsContent(t *testing.T) {
	var got struct {
		Value []map[string]any `json:"value"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_, _ = io.WriteString(w, `{"value":[{"key":"c1","status":true,"statusCode":200}]}`)
	}))
	defer srv.Close()

	idx := newTestIndexer(srv.URL) // semantic=false, embedder=nil
	// Content is populated by the projection even in keyword mode; it must be stripped.
	if err := idx.UpsertBatch(context.Background(), []Doc{{CandidateID: "c1", Content: "should be dropped"}}); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	a := got.Value[0]
	if _, present := a["content"]; present {
		t.Errorf("keyword push must not include content, got %v", a["content"])
	}
	if _, present := a["content_vector"]; present {
		t.Error("keyword push must not include content_vector")
	}
}

// TestUpsertBatch_EmbedFailureFailsBatch confirms an embed error aborts the upsert
// rather than pushing vector-less (silently unsearchable) docs.
func TestUpsertBatch_EmbedFailureFailsBatch(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = io.WriteString(w, `{"value":[]}`)
	}))
	defer srv.Close()

	idx := &azureIndexer{endpoint: srv.URL, key: "k", index: "candidates",
		dims: 8, embedder: fakeEmbedder{err: errEmbed}, semantic: true, http: http.DefaultClient}
	err := idx.UpsertBatch(context.Background(), []Doc{{CandidateID: "c1", Content: "x"}})
	if err == nil {
		t.Fatal("expected error when embedding fails")
	}
	if called {
		t.Error("must not push docs when embedding failed")
	}
}

var errEmbed = errFake("embed boom")

type errFake string

func (e errFake) Error() string { return string(e) }

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
