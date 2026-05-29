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

const openAIAPIVersion = "2024-08-01-preview"

const scoringSystemPrompt = `You are an HR screening assistant for Thai retail. Given a candidate profile ` +
	`and a job's keywords, return a strict JSON object: {"skills_score":<0-20 int>,` +
	`"strengths":["3 short Thai bullet points"],"red_flags":["..."],"suggested_positions":["..."]}. ` +
	`Score skills_score on semantic match between the candidate's skills/experience and the keywords. ` +
	`Respond with JSON only.`

// azureLLM evaluates the qualitative scoring part via Azure OpenAI.
type azureLLM struct {
	endpoint   string
	key        string
	deployment string
	http       *http.Client
}

func newAzureLLM(cfg *config.Config) azureLLM {
	return azureLLM{
		endpoint:   strings.TrimRight(cfg.AzureOpenAIEndpoint, "/"),
		key:        cfg.AzureOpenAIKey,
		deployment: cfg.AzureOpenAIDeployment,
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

type llmJSON struct {
	SkillsScore        int      `json:"skills_score"`
	Strengths          []string `json:"strengths"`
	RedFlags           []string `json:"red_flags"`
	SuggestedPositions []string `json:"suggested_positions"`
}

func (a azureLLM) evaluate(ctx context.Context, p ai.Profile, jd JD) (LLMPart, error) {
	profileJSON, _ := json.Marshal(p)
	user := fmt.Sprintf("Job keywords: %s\n\nCandidate profile:\n%s", strings.Join(jd.Keywords, ", "), string(profileJSON))

	body, err := json.Marshal(chatRequest{
		Messages: []chatMessage{
			{Role: "system", Content: scoringSystemPrompt},
			{Role: "user", Content: user},
		},
		Temperature:    0,
		MaxTokens:      500,
		ResponseFormat: map[string]string{"type": "json_object"},
	})
	if err != nil {
		return LLMPart{}, fmt.Errorf("scoring: marshal: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", a.endpoint, a.deployment, openAIAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return LLMPart{}, fmt.Errorf("scoring: request: %w", err)
	}
	req.Header.Set("api-key", a.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.http.Do(req)
	if err != nil {
		return LLMPart{}, fmt.Errorf("scoring: call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return LLMPart{}, fmt.Errorf("scoring: status %d: %s", resp.StatusCode, string(raw))
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return LLMPart{}, fmt.Errorf("scoring: decode: %w", err)
	}
	if len(cr.Choices) == 0 {
		return LLMPart{}, fmt.Errorf("scoring: no choices")
	}
	var parsed llmJSON
	if err := json.Unmarshal([]byte(cr.Choices[0].Message.Content), &parsed); err != nil {
		return LLMPart{}, fmt.Errorf("scoring: content json: %w", err)
	}
	return LLMPart(parsed), nil
}
