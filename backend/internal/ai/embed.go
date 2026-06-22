package ai

import (
	"context"

	"github.com/nexto/hr-ats/pkg/config"
)

// Embedder turns text into dense vector embeddings for semantic search. One
// vector per input string, in input order. The search package defines its own
// structurally-identical interface so it never imports ai (no cycle); this is
// the concrete side, selected by config.
type Embedder interface {
	// Embed returns one vector per input text, aligned by index. The slice
	// length always equals len(texts); an error means none were produced.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// NewEmbedder selects the embeddings client by config. Returns nil when semantic
// search is off (no AZURE_OPENAI_EMBED_DEPLOYMENT) — callers treat a nil Embedder
// as "keyword-only", so the search index/query degrade gracefully.
func NewEmbedder(cfg *config.Config) Embedder {
	if !cfg.UsesSemanticSearch() {
		return nil
	}
	return NewAzureEmbedder(
		cfg.AzureOpenAIEndpoint,
		cfg.AzureOpenAIKey,
		cfg.AzureOpenAIEmbedDeployment,
		cfg.AzureOpenAIEmbedDims,
	)
}

// mockEmbedder produces deterministic, dimension-correct vectors with no network
// call — for local/CI wiring and unit tests of the index/query embed paths.
type mockEmbedder struct{ dims int }

// NewMockEmbedder returns a deterministic Embedder of the given dimensionality.
func NewMockEmbedder(dims int) Embedder {
	if dims <= 0 {
		dims = 1536
	}
	return mockEmbedder{dims: dims}
}

func (m mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v := make([]float32, m.dims)
		// A cheap, stable hash spread across the dimensions so distinct texts get
		// distinct vectors (enough for tests; never used in production).
		var h uint32 = 2166136261
		for _, b := range []byte(t) {
			h = (h ^ uint32(b)) * 16777619
		}
		for j := range v {
			h = h*1664525 + 1013904223
			v[j] = float32(h%2000)/1000 - 1 // in [-1, 1)
		}
		out[i] = v
	}
	return out, nil
}
