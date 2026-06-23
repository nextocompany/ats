package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexto/hr-ats/pkg/config"
)

func TestProfileValidate(t *testing.T) {
	if err := (Profile{}).Validate(); err == nil {
		t.Error("expected error for profile with empty name")
	}
	p := Profile{Personal: Personal{Name: "สมชาย"}}
	if err := p.Validate(); err != nil {
		t.Errorf("unexpected error for valid profile: %v", err)
	}
}

// TestProfileIsResumeDefaultsTrue locks the false-positive-safety contract: a
// real CV must NEVER be flagged non-resume just because the LLM omitted the key.
func TestProfileIsResumeDefaultsTrue(t *testing.T) {
	cases := []struct {
		name string
		json string
		want bool
	}{
		{"key absent → true (bias to resume)", `{"personal":{"name":"สมชาย"}}`, true},
		{"explicit true", `{"personal":{"name":"สมชาย"},"is_resume":true}`, true},
		{"explicit false (non-resume)", `{"personal":{"name":""},"is_resume":false}`, false},
		{"null → true", `{"personal":{"name":"สมชาย"},"is_resume":null}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var p Profile
			if err := json.Unmarshal([]byte(c.json), &p); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if p.IsResume != c.want {
				t.Errorf("IsResume = %v, want %v", p.IsResume, c.want)
			}
		})
	}
}

// TestProfileUnmarshalKeepsFields guards the custom Profile.UnmarshalJSON
// embedded-alias path: adding is_resume must not drop the nested fields.
func TestProfileUnmarshalKeepsFields(t *testing.T) {
	const j = `{"personal":{"name":"สมชาย","age":"28"},
		"experience":[{"company":"Retail Co","position":"Cashier","duration_months":"24"}],
		"skills":["pos"],"is_resume":true}`
	var p Profile
	if err := json.Unmarshal([]byte(j), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Personal.Name != "สมชาย" || p.Personal.Age != 28 {
		t.Errorf("personal not parsed through alias: %+v", p.Personal)
	}
	if len(p.Experience) != 1 || p.Experience[0].DurationMonths != 24 {
		t.Errorf("experience not parsed through alias: %+v", p.Experience)
	}
	if !p.IsResume {
		t.Error("is_resume should be true")
	}
}

// TestMockParserNonResumeTrigger verifies the deterministic non-resume path used
// by the pipeline test + local runs.
func TestMockParserNonResumeTrigger(t *testing.T) {
	p := NewMockParser()
	got, err := p.Parse(context.Background(), "an invoice "+MockNonResumeMarker, "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.IsResume {
		t.Error("expected IsResume=false for the non-resume marker")
	}
	ok, err := p.Parse(context.Background(), strings.Repeat("resume ", 3), "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !ok.IsResume {
		t.Error("expected IsResume=true for normal text")
	}
}

func TestFactory_MockByDefault(t *testing.T) {
	ocr, parser := New(&config.Config{AIProvider: "mock"})

	res, err := ocr.Extract(context.Background(), []byte("pdf-bytes"), "pdf")
	if err != nil {
		t.Fatalf("mock ocr error: %v", err)
	}
	if res.Confidence < 0.9 {
		t.Errorf("expected high mock confidence, got %v", res.Confidence)
	}

	profile, err := parser.Parse(context.Background(), res.Text, "")
	if err != nil {
		t.Fatalf("mock parse error: %v", err)
	}
	if err := profile.Validate(); err != nil {
		t.Errorf("mock profile invalid: %v", err)
	}
}

func TestFactory_AzureSelected(t *testing.T) {
	// Construction only — no network call. Confirms the azure branch wires up.
	ocr, parser := New(&config.Config{
		AIProvider:            config.AIProviderAzure,
		AzureDocIntelEndpoint: "https://x.cognitiveservices.azure.com",
		AzureDocIntelKey:      "k",
		AzureOpenAIEndpoint:   "https://x.openai.azure.com",
		AzureOpenAIKey:        "k",
		AzureOpenAIDeployment: "hr-screening-gpt4o",
	})
	if _, ok := ocr.(azureOCR); !ok {
		t.Error("expected azureOCR implementation")
	}
	if _, ok := parser.(azureParser); !ok {
		t.Error("expected azureParser implementation")
	}
}
