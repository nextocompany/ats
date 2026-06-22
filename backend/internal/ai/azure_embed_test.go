package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

// embedServer builds a test server that returns dims-length vectors for each
// input, optionally failing the first n calls with 429 to exercise retry.
func embedServer(t *testing.T, dims, fail429 int) (*httptest.Server, *embedStats) {
	t.Helper()
	st := &embedStats{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req embedRequest
		_ = json.Unmarshal(raw, &req)

		st.mu.Lock()
		st.calls++
		st.lastDims = req.Dimensions
		st.batchSizes = append(st.batchSizes, len(req.Input))
		n := st.calls
		st.mu.Unlock()

		if n <= fail429 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":"throttled"}`)
			return
		}
		// Echo embeddings out of order to prove the client re-orders by index.
		var b []byte
		b = append(b, []byte(`{"data":[`)...)
		for i := len(req.Input) - 1; i >= 0; i-- {
			if i < len(req.Input)-1 {
				b = append(b, ',')
			}
			vec := make([]float32, dims)
			for j := range vec {
				vec[j] = float32(i)
			}
			vj, _ := json.Marshal(vec)
			b = append(b, []byte(`{"index":`+strconv.Itoa(i)+`,"embedding":`)...)
			b = append(b, vj...)
			b = append(b, '}')
		}
		b = append(b, []byte(`]}`)...)
		_, _ = w.Write(b)
	}))
	return srv, st
}

type embedStats struct {
	mu         sync.Mutex
	calls      int
	lastDims   int
	batchSizes []int
}

func newTestEmbedder(url string, dims int) azureEmbedder {
	return azureEmbedder{endpoint: url, key: "k", deployment: "embed", dims: dims, http: &http.Client{Timeout: 5 * time.Second}}
}

func TestAzureEmbedder_SendsDimensionsAndReorders(t *testing.T) {
	srv, st := embedServer(t, 4, 0)
	defer srv.Close()

	e := newTestEmbedder(srv.URL, 4)
	vecs, err := e.Embed(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 3 {
		t.Fatalf("got %d vectors, want 3", len(vecs))
	}
	if st.lastDims != 4 {
		t.Errorf("dimensions sent = %d, want 4 (must pin so -large can't break the index)", st.lastDims)
	}
	// Re-ordered by index: doc 0 → all 0s, doc 2 → all 2s.
	if vecs[0][0] != 0 || vecs[2][0] != 2 {
		t.Errorf("vectors not re-ordered by index: v0=%v v2=%v", vecs[0][0], vecs[2][0])
	}
}

func TestAzureEmbedder_SubBatches(t *testing.T) {
	srv, st := embedServer(t, 2, 0)
	defer srv.Close()

	e := newTestEmbedder(srv.URL, 2)
	texts := make([]string, embedBatchSize+5) // forces 2 sub-batches
	for i := range texts {
		texts[i] = "t"
	}
	vecs, err := e.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("got %d vectors, want %d", len(vecs), len(texts))
	}
	if st.calls != 2 {
		t.Errorf("calls = %d, want 2 sub-batches", st.calls)
	}
	if st.batchSizes[0] != embedBatchSize || st.batchSizes[1] != 5 {
		t.Errorf("batch sizes = %v, want [%d 5]", st.batchSizes, embedBatchSize)
	}
}

func TestAzureEmbedder_RetriesOn429(t *testing.T) {
	srv, st := embedServer(t, 2, 1) // first call 429, then succeeds
	defer srv.Close()

	e := newTestEmbedder(srv.URL, 2)
	vecs, err := e.Embed(context.Background(), []string{"a"})
	if err != nil {
		t.Fatalf("Embed should retry past 429: %v", err)
	}
	if len(vecs) != 1 {
		t.Fatalf("got %d vectors, want 1", len(vecs))
	}
	if st.calls != 2 {
		t.Errorf("calls = %d, want 2 (one 429 + one success)", st.calls)
	}
}

func TestMockEmbedder_DeterministicDims(t *testing.T) {
	m := NewMockEmbedder(16)
	a, err := m.Embed(context.Background(), []string{"x", "y"})
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 2 || len(a[0]) != 16 {
		t.Fatalf("shape wrong: %d x %d", len(a), len(a[0]))
	}
	b, _ := m.Embed(context.Background(), []string{"x"})
	if a[0][0] != b[0][0] {
		t.Error("mock embedder not deterministic for same input")
	}
}
