package ai

import "context"

// OCRResult is the output of the OCR step.
type OCRResult struct {
	// Text is the extracted document text (markdown).
	Text string
	// Confidence is the overall extraction confidence in [0,1].
	Confidence float64
}

// OCR extracts text from a raw resume file (pdf/docx/image).
type OCR interface {
	Extract(ctx context.Context, file []byte, fileType string) (OCRResult, error)
}
