package fit

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

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/pkg/config"
)

const openAIAPIVersion = "2024-08-01-preview"

// maxTurnChars bounds a single transcript turn fed into the prompt.
const maxTurnChars = 2000

// azureSummarizer calls Azure OpenAI, reusing the same deployment and HTTP shape
// as scoring.azureLLM and interview.azureInterviewer.
type azureSummarizer struct {
	endpoint   string
	key        string
	deployment string
	http       *http.Client
}

func newAzureSummarizer(cfg *config.Config) azureSummarizer {
	return azureSummarizer{
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
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

const fitSystemPrompt = `You are an HR talent-placement assistant for a Thai retail organization. ` +
	`Given a candidate's CV-screening result, their completed AI pre-interview evaluation, and a catalogue of Master JD positions ` +
	`(each with an id, title, responsibilities, and qualifications), decide which position(s) the candidate fits best across the WHOLE organization — ` +
	`not only the role they applied to. Return a strict JSON object: ` +
	`{"overall_fit":"strong|moderate|weak|none","summary":"2-3 Thai sentences","strengths":["Thai bullet points"],` +
	`"concerns":["Thai bullet points"],"recommended":[{"position_id":"<an id copied EXACTLY from the catalogue>","title":"<Thai title>","fit_score":<0-100 int>,"reasons":["Thai bullet points grounded in the position's responsibilities/qualifications"]}],` +
	`"no_match_reason":"Thai sentence"}. ` +
	`"strengths": list ONLY genuine positives the candidate actually HAS — each item must be something they possess, never something they lack. ` +
	`Return an empty array [] if there are none, and NEVER phrase a gap, a missing skill, or a weakness as a strength. ` +
	`"concerns": put every gap, missing qualification, mismatch, or weakness here instead. ` +
	`Rank recommended best-first. Only use position_id values that appear in the catalogue. ` +
	`If the candidate fits NO position, set "overall_fit":"none", "recommended":[], and explain why in "no_match_reason" (otherwise leave it empty). ` +
	`Ground every judgement in the screening result, the interview, and the specific responsibilities/qualifications. Respond with JSON only.`

func (a azureSummarizer) Summarize(ctx context.Context, in Inputs) (Analysis, error) {
	content, err := a.call(ctx, chatRequest{
		Messages: []chatMessage{
			{Role: "system", Content: fitSystemPrompt},
			{Role: "user", Content: buildUserPrompt(in)},
		},
		Temperature:    0,
		MaxTokens:      2000,
		ResponseFormat: map[string]string{"type": "json_object"},
	})
	if err != nil {
		return Analysis{}, err
	}
	a2, err := parseFit(content, in.Positions)
	if err != nil {
		return Analysis{}, err
	}
	a2.Model = "azure:" + a.deployment
	return a2, nil
}

// buildUserPrompt assembles the candidate context + the position catalogue.
func buildUserPrompt(in Inputs) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Candidate: %s\n\n", fallback(in.CandidateName, "(unknown)"))

	b.WriteString("== CV screening ==\n")
	fmt.Fprintf(&b, "Score: %s\n", scoreStr(in.ScreeningScore))
	fmt.Fprintf(&b, "Strengths/summary: %s\n", fallback(in.ScreeningSummary, "(none)"))
	fmt.Fprintf(&b, "Red flags: %s\n\n", fallback(in.ScreeningRedFlags, "(none)"))

	b.WriteString("== AI pre-interview ==\n")
	fmt.Fprintf(&b, "Score: %s\n", scoreStr(in.InterviewScore))
	fmt.Fprintf(&b, "Summary: %s\n", fallback(in.InterviewSummary, "(none)"))
	if len(in.InterviewStrengths) > 0 {
		fmt.Fprintf(&b, "Strengths: %s\n", strings.Join(in.InterviewStrengths, "; "))
	}
	if len(in.InterviewConcerns) > 0 {
		fmt.Fprintf(&b, "Concerns: %s\n", strings.Join(in.InterviewConcerns, "; "))
	}
	if len(in.Transcript) > 0 {
		b.WriteString("Transcript:\n")
		for _, t := range in.Transcript {
			who := "Interviewer"
			if t.Role == "user" {
				who = "Candidate"
			}
			// Cap each turn so an over-long (or adversarial) candidate answer can't
			// blow the token budget. Treat transcript text as untrusted content.
			fmt.Fprintf(&b, "%s: %s\n", who, truncate(t.Content, maxTurnChars))
		}
	}

	b.WriteString("\n== Master JD position catalogue ==\n")
	for _, p := range in.Positions {
		fmt.Fprintf(&b, "position_id: %s | %s\nResponsibilities: %s\nQualifications: %s\n\n",
			p.ID, p.Title, fallback(p.Responsibilities, "(not specified)"), fallback(p.Qualifications, "(not specified)"))
	}
	return b.String()
}

// call posts a chat-completion request and returns the first choice's content.
func (a azureSummarizer) call(ctx context.Context, cr chatRequest) (string, error) {
	body, err := json.Marshal(cr)
	if err != nil {
		return "", fmt.Errorf("fit: marshal: %w", err)
	}
	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", a.endpoint, a.deployment, openAIAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("fit: request: %w", err)
	}
	req.Header.Set("api-key", a.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fit: call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("fit: read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fit: status %d: %s", resp.StatusCode, string(raw))
	}
	var resps chatResponse
	if err := json.Unmarshal(raw, &resps); err != nil {
		return "", fmt.Errorf("fit: decode: %w", err)
	}
	if len(resps.Choices) == 0 {
		return "", fmt.Errorf("fit: no choices")
	}
	return resps.Choices[0].Message.Content, nil
}

// recJSON / fitJSON tolerate the LLM emitting fit_score as a number OR a string
// (gpt-4o-mini occasionally does the latter — see the resume/interview int-parse
// fixes). json.Number accepts both forms.
type recJSON struct {
	PositionID string      `json:"position_id"`
	Title      string      `json:"title"`
	FitScore   json.Number `json:"fit_score"`
	Reasons    []string    `json:"reasons"`
}

type fitJSON struct {
	OverallFit    string    `json:"overall_fit"`
	Summary       string    `json:"summary"`
	Strengths     []string  `json:"strengths"`
	Concerns      []string  `json:"concerns"`
	Recommended   []recJSON `json:"recommended"`
	NoMatchReason string    `json:"no_match_reason"`
}

// parseFit decodes the LLM JSON and sanitises it: it normalises overall_fit,
// clamps fit_score to [0,100], coerces number-or-string scores, and DROPS any
// recommended position whose id is not present in the supplied catalogue (guards
// against a hallucinated position_id).
func parseFit(content string, catalogue []PositionCard) (Analysis, error) {
	var parsed fitJSON
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return Analysis{}, fmt.Errorf("fit: parse json: %w", err)
	}

	valid := make(map[uuid.UUID]string, len(catalogue))
	for _, p := range catalogue {
		valid[p.ID] = p.Title
	}

	out := Analysis{
		OverallFit:    normalizeOverall(parsed.OverallFit),
		Summary:       parsed.Summary,
		Strengths:     nonNilStr(parsed.Strengths),
		Concerns:      nonNilStr(parsed.Concerns),
		NoMatchReason: parsed.NoMatchReason,
		Recommended:   []RecommendedPosition{},
	}

	for _, r := range parsed.Recommended {
		id, err := uuid.Parse(strings.TrimSpace(r.PositionID))
		if err != nil {
			continue // not a uuid → skip
		}
		title, ok := valid[id]
		if !ok {
			continue // hallucinated id not in the catalogue → drop
		}
		out.Recommended = append(out.Recommended, RecommendedPosition{
			PositionID: id,
			Title:      fallback(r.Title, title),
			FitScore:   clampScore(r.FitScore),
			Reasons:    nonNilStr(r.Reasons),
		})
	}

	// Keep the verdict and the list consistent: no usable recommendation ⇒ none.
	if len(out.Recommended) == 0 && out.OverallFit != OverallNone {
		out.OverallFit = OverallNone
		if out.NoMatchReason == "" {
			out.NoMatchReason = "ไม่พบตำแหน่งใน Master JD ที่เหมาะสมกับผู้สมัครรายนี้"
		}
	}
	return out, nil
}

func normalizeOverall(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case OverallStrong:
		return OverallStrong
	case OverallModerate:
		return OverallModerate
	case OverallNone:
		return OverallNone
	default:
		return OverallWeak
	}
}

func clampScore(n json.Number) int {
	f, err := strconv.ParseFloat(strings.TrimSpace(n.String()), 64)
	if err != nil {
		return 0
	}
	if f < 0 {
		f = 0
	}
	if f > 100 {
		f = 100
	}
	return int(f)
}

func scoreStr(p *float64) string {
	if p == nil {
		return "(not scored)"
	}
	return strconv.FormatFloat(*p, 'f', 0, 64)
}

func fallback(v, dflt string) string {
	if strings.TrimSpace(v) == "" {
		return dflt
	}
	return v
}

// truncate caps s to at most n runes, appending an ellipsis when it cuts.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
