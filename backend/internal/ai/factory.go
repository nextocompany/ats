package ai

import "github.com/nexto/hr-ats/pkg/config"

// New returns the OCR and Parser implementations selected by configuration.
// This is the single place provider choice lives; mock is the default so local
// and CI runs require no Azure credentials.
func New(cfg *config.Config) (OCR, Parser) {
	if cfg.UsesGeminiAI() {
		return NewGeminiOCR(cfg.GeminiAPIKey, cfg.GeminiModel),
			NewGeminiParser(cfg.GeminiAPIKey, cfg.GeminiModel)
	}
	if cfg.UsesAzureAI() {
		return NewAzureOCR(cfg.AzureDocIntelEndpoint, cfg.AzureDocIntelKey),
			NewAzureParser(cfg.AzureOpenAIEndpoint, cfg.AzureOpenAIKey, cfg.AzureOpenAIDeployment)
	}
	return NewMockOCR(), NewMockParser()
}
