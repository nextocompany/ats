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

const openAIAPIVersion = "2024-08-01-preview"

const cvParserSystemPrompt = `You are a Thai/English CV parser. Extract the candidate's details from the ` +
	`provided resume text into a strict JSON object matching this schema (use empty strings/arrays/0 when ` +
	`unknown): {"personal":{"name","phone","email","address","age","id_card"},` +
	`"experience":[{"company","position","duration_months","description"}],` +
	`"education":[{"degree","major","institution","year"}],"skills":[],` +
	`"languages":[{"language","level"}],"desired_position","is_resume"}. ` +
	`Set "is_resume" to true if the document is a resume/CV, or false ONLY when it is clearly NOT a ` +
	`resume (e.g. an invoice, receipt, ID card/photo, or an unrelated document). When uncertain, set true. ` +
	`Respond with JSON only.`

// azureParser calls Azure OpenAI (GPT-4o) chat completions over REST.
type azureParser struct {
	endpoint   string
	key        string
	deployment string
	http       *http.Client
}

// NewAzureParser builds the Azure OpenAI parser client.
func NewAzureParser(endpoint, key, deployment string) Parser {
	return azureParser{
		endpoint:   strings.TrimRight(endpoint, "/"),
		key:        key,
		deployment: deployment,
		http:       &http.Client{Timeout: 60 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Messages       []chatMessage     `json:"messages"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens"`
	ResponseFormat map[string]string `json:"response_format"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (a azureParser) Parse(ctx context.Context, text, positionContext string) (Profile, error) {
	user := text
	if positionContext != "" {
		user = "Applied position context: " + positionContext + "\n\nResume:\n" + text
	}

	reqBody := chatRequest{
		Messages: []chatMessage{
			{Role: "system", Content: cvParserSystemPrompt},
			{Role: "user", Content: user},
		},
		Temperature:    0,
		MaxTokens:      2000,
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return Profile{}, fmt.Errorf("ai: openai marshal: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		a.endpoint, a.deployment, openAIAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return Profile{}, fmt.Errorf("ai: openai request: %w", err)
	}
	req.Header.Set("api-key", a.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.http.Do(req)
	if err != nil {
		return Profile{}, fmt.Errorf("ai: openai call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return Profile{}, fmt.Errorf("ai: openai status %d: %s", resp.StatusCode, string(body))
	}

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return Profile{}, fmt.Errorf("ai: openai decode: %w", err)
	}
	if len(cr.Choices) == 0 {
		return Profile{}, fmt.Errorf("ai: openai returned no choices")
	}

	var profile Profile
	if err := json.Unmarshal([]byte(cr.Choices[0].Message.Content), &profile); err != nil {
		return Profile{}, fmt.Errorf("ai: openai content not valid profile json: %w", err)
	}
	// Validation is the pipeline's responsibility: a non-resume (is_resume=false,
	// empty name) must reach the caller as a successful parse, not be swallowed here
	// as an error (which would be indistinguishable from a transient LLM failure).
	return profile, nil
}
