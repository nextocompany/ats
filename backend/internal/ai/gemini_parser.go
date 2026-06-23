package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const geminiParserSystemPrompt = `You are a Thai/English CV parser. Extract the candidate's details from the ` +
	`provided resume text into a strict JSON object matching this schema (use empty strings/arrays/0 when ` +
	`unknown): {"personal":{"name","phone","email","address","age","id_card"},` +
	`"experience":[{"company","position","duration_months","description"}],` +
	`"education":[{"degree","major","institution","year"}],"skills":[],` +
	`"languages":[{"language","level"}],"desired_position","is_resume"}. ` +
	`Set "is_resume" to true if the document is a resume/CV, or false ONLY when it is clearly NOT a ` +
	`resume (e.g. an invoice, receipt, ID card/photo, or an unrelated document). When uncertain, set true. ` +
	`Respond with JSON only.`

// geminiParser turns OCR text into a structured Profile via the Gemini REST API
// using JSON response mode.
type geminiParser struct {
	client geminiClient
}

// NewGeminiParser builds the Gemini CV parser client.
func NewGeminiParser(apiKey, model string) Parser {
	return geminiParser{client: newGeminiClient(apiKey, model, 60*time.Second)}
}

func (p geminiParser) Parse(ctx context.Context, text, positionContext string) (Profile, error) {
	user := text
	if positionContext != "" {
		user = "Applied position context: " + positionContext + "\n\nResume:\n" + text
	}

	body := geminiRequest{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: geminiParserSystemPrompt}}},
		Contents: []geminiContent{{
			Role:  "user",
			Parts: []geminiPart{{Text: user}},
		}},
		GenerationConfig: geminiGenerationConfig{Temperature: 0, ResponseMimeType: "application/json"},
	}

	content, err := p.client.generate(ctx, body)
	if err != nil {
		return Profile{}, err
	}

	var profile Profile
	if err := json.Unmarshal([]byte(content), &profile); err != nil {
		return Profile{}, fmt.Errorf("ai: gemini content not valid profile json: %w", err)
	}
	// Validation is the pipeline's responsibility: a non-resume (is_resume=false,
	// empty name) must reach the caller as a successful parse, not be swallowed here
	// as an error (indistinguishable from a transient LLM failure).
	return profile, nil
}
