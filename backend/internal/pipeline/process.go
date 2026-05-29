// Package pipeline implements the asynq task handler for the AI processing
// pipeline. Sprint 1 covers Steps 1–2 (OCR → CV parse → persist). Steps 3–7
// (dedup, score, gate, branch-assign, notify) extend this handler in Sprint 2+.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/ai"
	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/pkg/queue"
)

// ocrConfidenceThreshold flags low-confidence extractions for manual review
// without aborting the pipeline (PRP §8 Step 1 fallback).
const ocrConfidenceThreshold = 0.7

// BlobStore is the subset of blob.Client the pipeline needs.
type BlobStore interface {
	Upload(ctx context.Context, name string, data []byte, contentType string) (string, error)
	Download(ctx context.Context, name string) ([]byte, error)
}

// Processor holds the dependencies for the process_application task.
type Processor struct {
	ocr        ai.OCR
	parser     ai.Parser
	blob       BlobStore
	candidates candidates.Repository
	apps       applications.Repository
}

// NewProcessor wires the pipeline processor.
func NewProcessor(o ai.OCR, p ai.Parser, b BlobStore, c candidates.Repository, a applications.Repository) *Processor {
	return &Processor{ocr: o, parser: p, blob: b, candidates: c, apps: a}
}

// HandleProcessApplication runs OCR + parse for one application. Returning an
// error lets asynq retry; once retries are exhausted the task is marked failed
// by the caller's retention. We also set the row status to failed on hard
// errors so the record reflects the outcome.
func (pr *Processor) HandleProcessApplication(ctx context.Context, t *asynq.Task) error {
	payload, err := queue.ParseProcessApplicationPayload(t.Payload())
	if err != nil {
		return err // malformed payload — not retryable, but asynq will move to archive
	}

	appID, err := uuid.Parse(payload.ApplicationID)
	if err != nil {
		return fmt.Errorf("pipeline: bad application id: %w", err)
	}
	candID, err := uuid.Parse(payload.CandidateID)
	if err != nil {
		return fmt.Errorf("pipeline: bad candidate id: %w", err)
	}

	logger := log.With().
		Str("application_id", payload.ApplicationID).
		Str("blob", payload.BlobName).
		Logger()

	if err := pr.run(ctx, payload, appID, candID, logger); err != nil {
		logger.Error().Err(err).Msg("pipeline failed")
		// Best-effort: reflect failure on the row (asynq still controls retries).
		if serr := pr.apps.SetStatus(ctx, appID, applications.StatusFailed); serr != nil {
			logger.Error().Err(serr).Msg("failed to mark application failed")
		}
		return err
	}
	return nil
}

func (pr *Processor) run(ctx context.Context, p queue.ProcessApplicationPayload, appID, candID uuid.UUID, logger zerolog.Logger) error {
	// Download raw file.
	raw, err := pr.blob.Download(ctx, p.BlobName)
	if err != nil {
		return fmt.Errorf("pipeline: download: %w", err)
	}

	// Step 1 — OCR.
	ocrRes, err := pr.ocr.Extract(ctx, raw, p.FileType)
	if err != nil {
		return fmt.Errorf("pipeline: ocr: %w", err)
	}
	needsReview := ocrRes.Confidence < ocrConfidenceThreshold
	if needsReview {
		logger.Warn().Float64("confidence", ocrRes.Confidence).Msg("low OCR confidence — flagging manual review")
	}

	// Persist OCR text (idempotent by key).
	ocrName := fmt.Sprintf("ocr/%s/text.md", appID)
	ocrURL, err := pr.blob.Upload(ctx, ocrName, []byte(ocrRes.Text), "text/markdown")
	if err != nil {
		return fmt.Errorf("pipeline: upload ocr text: %w", err)
	}

	// Step 2 — CV parse.
	profile, err := pr.parser.Parse(ctx, ocrRes.Text, p.PositionID)
	if err != nil {
		return fmt.Errorf("pipeline: parse: %w", err)
	}
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("pipeline: invalid profile: %w", err)
	}

	// Persist parsed profile JSON.
	profileJSON, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("pipeline: marshal profile: %w", err)
	}
	profileName := fmt.Sprintf("profiles/%s/parsed_profile.json", appID)
	profileURL, err := pr.blob.Upload(ctx, profileName, profileJSON, "application/json")
	if err != nil {
		return fmt.Errorf("pipeline: upload profile: %w", err)
	}

	// Write structured fields back to the candidate.
	if err := pr.candidates.UpdateProfileFields(ctx, candID, candidates.ProfileFields{
		FullName: profile.Personal.Name,
		Phone:    profile.Personal.Phone,
		Email:    profile.Personal.Email,
		Address:  profile.Personal.Address,
	}); err != nil {
		return fmt.Errorf("pipeline: update candidate: %w", err)
	}

	// Mark the application parsed.
	if err := pr.apps.SetParseResults(ctx, appID, applications.ParseResult{
		OCRTextBlobURL:       ocrURL,
		ParsedProfileBlobURL: profileURL,
		OCRConfidence:        ocrRes.Confidence,
		NeedsManualReview:    needsReview,
	}); err != nil {
		return fmt.Errorf("pipeline: set parse results: %w", err)
	}

	logger.Info().Bool("needs_manual_review", needsReview).Msg("application parsed")
	return nil
}
