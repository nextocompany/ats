package ai

import (
	"context"
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
