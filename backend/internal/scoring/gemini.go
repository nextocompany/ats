package scoring

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nexto/hr-ats/internal/ai"
	"github.com/nexto/hr-ats/pkg/config"
)

// geminiEndpoint is the Generative Language API generateContent base; the model
// name is interpolated per request.
const geminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"

// geminiLLM evaluates the qualitative scoring part via the Google Gemini REST
// API. It reuses scoringSystemPrompt (shared with the Azure path) so scoring
// intent is identical across providers.
type geminiLLM struct {
	apiKey string
	model  string
	http   *http.Client
}

func newGeminiLLM(cfg *config.Config) geminiLLM {
	return geminiLLM{
		apiKey: cfg.GeminiAPIKey,
		model:  cfg.GeminiModel,
		http:   &http.Client{Timeout: 60 * time.Second},
	}
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiGenerationConfig struct {
	Temperature      float64 `json:"temperature"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent         `json:"systemInstruction,omitempty"`
	Contents          []geminiContent        `json:"contents"`
	GenerationConfig  geminiGenerationConfig `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
}

func (g geminiLLM) evaluate(ctx context.Context, p ai.Profile, jd JD) (LLMPart, error) {
	profileJSON, _ := json.Marshal(p)
	user := fmt.Sprintf("Job description:\n%s\n\nCandidate profile:\n%s", jd.promptText(), string(profileJSON))

	body, err := json.Marshal(geminiRequest{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: scoringSystemPrompt}}},
		Contents: []geminiContent{{
			Role:  "user",
			Parts: []geminiPart{{Text: user}},
		}},
		GenerationConfig: geminiGenerationConfig{Temperature: 0, ResponseMimeType: "application/json"},
	})
	if err != nil {
		return LLMPart{}, fmt.Errorf("scoring: marshal: %w", err)
	}

	url := fmt.Sprintf(geminiEndpoint, g.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return LLMPart{}, fmt.Errorf("scoring: request: %w", err)
	}
	req.Header.Set("x-goog-api-key", g.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.http.Do(req)
	if err != nil {
		return LLMPart{}, fmt.Errorf("scoring: call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return LLMPart{}, fmt.Errorf("scoring: status %d: %s", resp.StatusCode, string(raw))
	}

	var gr geminiResponse
	if err := json.Unmarshal(raw, &gr); err != nil {
		return LLMPart{}, fmt.Errorf("scoring: decode: %w", err)
	}
	if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return LLMPart{}, fmt.Errorf("scoring: no candidates")
	}

	var content strings.Builder
	for _, part := range gr.Candidates[0].Content.Parts {
		content.WriteString(part.Text)
	}

	var parsed llmJSON
	if err := json.Unmarshal([]byte(content.String()), &parsed); err != nil {
		return LLMPart{}, fmt.Errorf("scoring: content json: %w", err)
	}
	return LLMPart(parsed), nil
}
