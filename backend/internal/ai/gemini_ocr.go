package ai

import (
	"context"
	"encoding/base64"
	"strings"
	"time"
)

const geminiOCRPrompt = `Transcribe ALL text from this document into clean Markdown. ` +
	`Preserve the original structure (headings, lists, tables) as faithfully as ` +
	`possible. Do not summarize, translate, or add commentary — output only the ` +
	`transcribed document text.`

// geminiOCR transcribes a resume file to Markdown via the Gemini REST API by
// sending the file as inline base64 data alongside the transcription prompt.
type geminiOCR struct {
	client geminiClient
}

// NewGeminiOCR builds the Gemini OCR client. OCR can be slow on large PDFs, so
// the timeout matches the Azure parser (60s).
func NewGeminiOCR(apiKey, model string) OCR {
	return geminiOCR{client: newGeminiClient(apiKey, model, 60*time.Second)}
}

// geminiMimeType maps a resume file extension/type to an inline-data MIME type.
// Gemini infers nothing from the bytes, so the caller must label them.
func geminiMimeType(fileType string) string {
	switch strings.ToLower(strings.TrimPrefix(fileType, ".")) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "pdf":
		return "application/pdf"
	default:
		return "application/pdf"
	}
}

func (o geminiOCR) Extract(ctx context.Context, file []byte, fileType string) (OCRResult, error) {
	encoded := base64.StdEncoding.EncodeToString(file)

	body := geminiRequest{
		Contents: []geminiContent{{
			Role: "user",
			Parts: []geminiPart{
				{InlineData: &geminiInlineData{MimeType: geminiMimeType(fileType), Data: encoded}},
				{Text: geminiOCRPrompt},
			},
		}},
		GenerationConfig: geminiGenerationConfig{Temperature: 0},
	}

	text, err := o.client.generate(ctx, body)
	if err != nil {
		return OCRResult{}, err
	}
	// Gemini provides no per-token confidence; report full confidence.
	return OCRResult{Text: text, Confidence: 1.0}, nil
}
