package ai

import "context"

// Parser turns extracted OCR text into a structured Profile.
type Parser interface {
	// Parse extracts a structured profile. positionContext is optional hint text
	// about the applied position to improve extraction.
	Parse(ctx context.Context, text, positionContext string) (Profile, error)
}
