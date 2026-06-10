package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// geminiEndpoint is the Generative Language API generateContent base. The model
// name is interpolated per request: .../models/{model}:generateContent.
const geminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"

// geminiPart is one content part: either text or inline (base64) binary data.
type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
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

// geminiClient is the shared REST transport for the ai-package Gemini providers.
type geminiClient struct {
	apiKey string
	model  string
	http   *http.Client
}

func newGeminiClient(apiKey, model string, timeout time.Duration) geminiClient {
	return geminiClient{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: timeout},
	}
}

// generate POSTs the request and returns the concatenated candidate text.
func (g geminiClient) generate(ctx context.Context, body geminiRequest) (string, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("ai: gemini marshal: %w", err)
	}

	url := fmt.Sprintf(geminiEndpoint, g.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("ai: gemini request: %w", err)
	}
	req.Header.Set("x-goog-api-key", g.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai: gemini call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ai: gemini status %d: %s", resp.StatusCode, string(respBody))
	}

	var gr geminiResponse
	if err := json.Unmarshal(respBody, &gr); err != nil {
		return "", fmt.Errorf("ai: gemini decode: %w", err)
	}
	if len(gr.Candidates) == 0 {
		return "", fmt.Errorf("ai: gemini returned no candidates")
	}

	var sb strings.Builder
	for _, part := range gr.Candidates[0].Content.Parts {
		sb.WriteString(part.Text)
	}
	return sb.String(), nil
}
