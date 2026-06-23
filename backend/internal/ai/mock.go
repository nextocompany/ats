package ai

import (
	"context"
	"fmt"
	"strings"
)

// MockNonResumeMarker, when present in the OCR text passed to the mock parser,
// makes it return a non-resume profile (is_resume=false). Lets dev/CI exercise
// the invalid-resume path deterministically without a real LLM.
const MockNonResumeMarker = "__NOT_A_RESUME__"

// mockOCR returns deterministic output so local runs and CI are reproducible.
type mockOCR struct{}

// NewMockOCR returns a deterministic OCR implementation for dev/CI.
func NewMockOCR() OCR { return mockOCR{} }

func (mockOCR) Extract(_ context.Context, file []byte, fileType string) (OCRResult, error) {
	return OCRResult{
		Text: fmt.Sprintf(
			"# Resume (mock OCR)\n\nfile_type: %s\nbytes: %d\n\nName: สมชาย ใจดี\nPhone: 0812345678\nEmail: somchai@example.com\nExperience: Cashier at Retail Co (24 months)\nEducation: ปวส. Business, Bangkok College, 2018\nSkills: cashier, customer service, POS",
			fileType, len(file),
		),
		Confidence: 0.95,
	}, nil
}

// mockParser returns a fixed valid profile derived deterministically from input.
type mockParser struct{}

// NewMockParser returns a deterministic Parser implementation for dev/CI.
func NewMockParser() Parser { return mockParser{} }

func (mockParser) Parse(_ context.Context, text, _ string) (Profile, error) {
	// Non-resume path: a document that isn't a CV parses successfully but with
	// is_resume=false and no identity (mirrors a real LLM on an invoice/photo).
	if strings.Contains(text, MockNonResumeMarker) {
		return Profile{IsResume: false}, nil
	}
	return Profile{
		Personal: Personal{
			Name:    "สมชาย ใจดี",
			Phone:   "0812345678",
			Email:   "somchai@example.com",
			Address: "Bangkok",
			Age:     28,
			IDCard:  "",
		},
		Experience: []Experience{
			{Company: "Retail Co", Position: "Cashier", DurationMonths: 24, Description: "POS and customer service"},
		},
		Education: []Education{
			{Degree: "ปวส.", Major: "Business", Institution: "Bangkok College", Year: 2018},
		},
		Skills:          []string{"cashier", "customer service", "POS"},
		Languages:       []Language{{Language: "Thai", Level: "native"}, {Language: "English", Level: "basic"}},
		DesiredPosition: "Cashier",
		// Explicit: a struct literal does NOT run UnmarshalJSON's default-true.
		IsResume: true,
	}, nil
}
