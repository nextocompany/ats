package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// embedBatchSize caps texts per embeddings request. Azure OpenAI embeddings
// tolerate large batches, but small sub-batches (16-64) keep each call well
// under per-request token limits and make 429 backoff cheap to retry. 32 is a
// safe middle ground for short candidate blurbs.
const embedBatchSize = 32

// embedMaxRetries bounds 429/503 retries per sub-batch. Recall-fit features have
// hit TPM limits on this shared OpenAI resource, so backoff is mandatory.
const embedMaxRetries = 5

// azureEmbedder calls Azure OpenAI embeddings over REST. It mirrors azureParser's
// endpoint/key/deployment shape and reuses the same api-version. dims is pinned in
// every request so a model swap (e.g. text-embedding-3-large) can't silently
// change the vector size and break the index.
type azureEmbedder struct {
	endpoint   string
	key        string
	deployment string
	dims       int
	http       *http.Client
}

// NewAzureEmbedder builds the Azure OpenAI embeddings client.
func NewAzureEmbedder(endpoint, key, deployment string, dims int) Embedder {
	return azureEmbedder{
		endpoint:   strings.TrimRight(endpoint, "/"),
		key:        key,
		deployment: deployment,
		dims:       dims,
		http:       &http.Client{Timeout: 60 * time.Second},
	}
}

type embedRequest struct {
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// Embed returns one vector per text, in input order, batching into sub-batches of
// embedBatchSize. Any sub-batch failure (after retries) fails the whole call so a
// caller never receives partially-embedded output.
func (a azureEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += embedBatchSize {
		end := start + embedBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		vecs, err := a.embedBatch(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, vecs...)
	}
	return out, nil
}

func (a azureEmbedder) embedBatch(ctx context.Context, batch []string) ([][]float32, error) {
	raw, err := json.Marshal(embedRequest{Input: batch, Dimensions: a.dims})
	if err != nil {
		return nil, fmt.Errorf("ai: embed marshal: %w", err)
	}
	url := fmt.Sprintf("%s/openai/deployments/%s/embeddings?api-version=%s",
		a.endpoint, a.deployment, openAIAPIVersion)

	var lastErr error
	retryAfter := 0 // server Retry-After hint carried between attempts
	for attempt := 0; attempt <= embedMaxRetries; attempt++ {
		if attempt > 0 {
			// Backoff honours Retry-After when present; otherwise exponential.
			if err := sleepCtx(ctx, a.backoff(attempt, retryAfter)); err != nil {
				return nil, err
			}
		}
		vecs, hint, err := a.doEmbed(ctx, url, raw, len(batch))
		if err == nil {
			return vecs, nil
		}
		lastErr = err
		if hint < 0 { // non-retryable
			return nil, err
		}
		retryAfter = hint
	}
	return nil, fmt.Errorf("ai: embed exhausted retries: %w", lastErr)
}

// doEmbed performs one request. The second return is the retry hint: -1 means do
// not retry (success or hard error), 0 means retry with default backoff, and >0
// is a server-suggested Retry-After in seconds.
func (a azureEmbedder) doEmbed(ctx context.Context, url string, body []byte, want int) ([][]float32, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, -1, fmt.Errorf("ai: embed request: %w", err)
	}
	req.Header.Set("api-key", a.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("ai: embed call: %w", err) // network error → retry
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		after := 0
		if ra, e := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After"))); e == nil && ra > 0 {
			after = ra
		}
		return nil, after, fmt.Errorf("ai: embed status %d: %s", resp.StatusCode, truncate(respBody))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, -1, fmt.Errorf("ai: embed status %d: %s", resp.StatusCode, truncate(respBody))
	}

	var er embedResponse
	if err := json.Unmarshal(respBody, &er); err != nil {
		return nil, -1, fmt.Errorf("ai: embed decode: %w", err)
	}
	if len(er.Data) != want {
		return nil, -1, fmt.Errorf("ai: embed returned %d vectors, want %d", len(er.Data), want)
	}
	// Re-order by index — the response is input-ordered, but pin it defensively.
	out := make([][]float32, want)
	for _, d := range er.Data {
		if d.Index < 0 || d.Index >= want {
			return nil, -1, fmt.Errorf("ai: embed index %d out of range", d.Index)
		}
		out[d.Index] = d.Embedding
	}
	for i, v := range out {
		if len(v) == 0 {
			return nil, -1, fmt.Errorf("ai: embed missing vector at index %d", i)
		}
	}
	return out, -1, nil
}

// backoff returns the wait before the given attempt (1-based). A server-provided
// Retry-After wins; otherwise exponential 250ms·2^(n-1) capped at 8s.
func (azureEmbedder) backoff(attempt, retryAfter int) time.Duration {
	if retryAfter > 0 {
		return time.Duration(retryAfter) * time.Second
	}
	d := 250 * time.Millisecond << (attempt - 1)
	if d > 8*time.Second {
		d = 8 * time.Second
	}
	return d
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func truncate(b []byte) string {
	const max = 512
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		return s[:max]
	}
	return s
}
