// Package pipeline implements the asynq task handler for the AI processing
// pipeline. Sprint 1 covered Steps 1–2 (OCR → parse). Sprint 2 adds Steps 3–6
// (dedup → score → must-have gate → branch assignment). Step 7 (notify) is
// Sprint 5.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/ai"
	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/branch"
	"github.com/nexto/hr-ats/internal/candidates"
	"github.com/nexto/hr-ats/internal/dedup"
	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/internal/scoring"
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

// CandidateIndexer keeps the search index in sync with a candidate's state. The
// interface lives here (not in internal/search) so the pipeline doesn't import
// search — avoiding an import cycle. The default is a no-op; the worker injects
// the real (search-backed) implementation via SetIndexer.
type CandidateIndexer interface {
	Index(ctx context.Context, candidateID uuid.UUID) error
}

type noopCandidateIndexer struct{}

func (noopCandidateIndexer) Index(context.Context, uuid.UUID) error { return nil }

// Processor holds the dependencies for the process_application task.
type Processor struct {
	ocr        ai.OCR
	parser     ai.Parser
	blob       BlobStore
	candidates candidates.Repository
	apps       applications.Repository
	dedup      *dedup.Service
	scorer     scoring.Scorer
	assigner   *branch.Assigner
	positions  positions.Repository
	indexer    CandidateIndexer
}

// NewProcessor wires the pipeline processor.
func NewProcessor(
	o ai.OCR, p ai.Parser, b BlobStore,
	c candidates.Repository, a applications.Repository,
	d *dedup.Service, s scoring.Scorer, asn *branch.Assigner, pos positions.Repository,
) *Processor {
	return &Processor{
		ocr: o, parser: p, blob: b, candidates: c, apps: a,
		dedup: d, scorer: s, assigner: asn, positions: pos,
		indexer: noopCandidateIndexer{},
	}
}

// SetIndexer injects a search indexer (no-op by default). Called by the worker
// when AI_SEARCH_PROVIDER=azure; ignored for nil so callers/tests stay simple.
func (pr *Processor) SetIndexer(idx CandidateIndexer) {
	if idx != nil {
		pr.indexer = idx
	}
}

// HandleProcessApplication runs the full pipeline for one application. Returning
// an error lets asynq retry; on a hard error we also mark the row failed.
func (pr *Processor) HandleProcessApplication(ctx context.Context, t *asynq.Task) error {
	payload, err := queue.ParseProcessApplicationPayload(t.Payload())
	if err != nil {
		return err
	}
	appID, err := uuid.Parse(payload.ApplicationID)
	if err != nil {
		return fmt.Errorf("pipeline: bad application id: %w", err)
	}
	candID, err := uuid.Parse(payload.CandidateID)
	if err != nil {
		return fmt.Errorf("pipeline: bad candidate id: %w", err)
	}
	positionID, err := uuid.Parse(payload.PositionID)
	if err != nil {
		return fmt.Errorf("pipeline: bad position id: %w", err)
	}

	logger := log.With().
		Str("application_id", payload.ApplicationID).
		Str("blob", payload.BlobName).
		Logger()

	if err := pr.run(ctx, payload, appID, candID, positionID, logger); err != nil {
		logger.Error().Err(err).Msg("pipeline failed")
		if serr := pr.apps.SetStatus(ctx, appID, applications.StatusFailed); serr != nil {
			logger.Error().Err(serr).Msg("failed to mark application failed")
		}
		return err
	}

	// Keep the search index fresh — best-effort. The candidate's searchable state
	// (status/score/store) is now final; a stale index must never fail the task.
	if err := pr.indexer.Index(ctx, candID); err != nil {
		logger.Warn().Err(err).Msg("search index update failed (non-fatal)")
	}
	return nil
}

func (pr *Processor) run(ctx context.Context, p queue.ProcessApplicationPayload, appID, candID, positionID uuid.UUID, logger zerolog.Logger) error {
	// Step 1 — OCR.
	raw, err := pr.blob.Download(ctx, p.BlobName)
	if err != nil {
		return fmt.Errorf("pipeline: download: %w", err)
	}
	ocrRes, err := pr.ocr.Extract(ctx, raw, p.FileType)
	if err != nil {
		return fmt.Errorf("pipeline: ocr: %w", err)
	}
	needsReview := ocrRes.Confidence < ocrConfidenceThreshold
	if needsReview {
		logger.Warn().Float64("confidence", ocrRes.Confidence).Msg("low OCR confidence — flagging manual review")
	}
	ocrURL, err := pr.blob.Upload(ctx, fmt.Sprintf("ocr/%s/text.md", appID), []byte(ocrRes.Text), "text/markdown")
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
	profileJSON, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("pipeline: marshal profile: %w", err)
	}
	profileURL, err := pr.blob.Upload(ctx, fmt.Sprintf("profiles/%s/parsed_profile.json", appID), profileJSON, "application/json")
	if err != nil {
		return fmt.Errorf("pipeline: upload profile: %w", err)
	}
	if err := pr.candidates.UpdateProfileFields(ctx, candID, candidates.ProfileFields{
		FullName: profile.Personal.Name,
		Phone:    profile.Personal.Phone,
		Email:    profile.Personal.Email,
		Address:  profile.Personal.Address,
	}); err != nil {
		return fmt.Errorf("pipeline: update candidate: %w", err)
	}
	if err := pr.apps.SetParseResults(ctx, appID, applications.ParseResult{
		OCRTextBlobURL:       ocrURL,
		ParsedProfileBlobURL: profileURL,
		OCRConfidence:        ocrRes.Confidence,
		NeedsManualReview:    needsReview,
	}); err != nil {
		return fmt.Errorf("pipeline: set parse results: %w", err)
	}

	// Step 3 — Dedup. Reconcile the just-created candidate against existing ones.
	decision, err := pr.dedup.Reconcile(ctx, candID, profile.Personal.Name, profile.Personal.Phone, profile.Personal.Email, profile.Personal.IDCard)
	if err != nil {
		return fmt.Errorf("pipeline: dedup: %w", err)
	}
	canonicalID := decision.CanonicalID
	if decision.State != dedup.StateNone {
		if err := pr.apps.SetDedupState(ctx, appID, decision.State, decision.Confidence); err != nil {
			return fmt.Errorf("pipeline: set dedup state: %w", err)
		}
	}
	if canonicalID != candID {
		if err := pr.apps.SetCanonicalCandidate(ctx, appID, canonicalID); err != nil {
			return fmt.Errorf("pipeline: repoint candidate: %w", err)
		}
		logger.Info().Str("canonical", canonicalID.String()).Str("state", decision.State).Msg("duplicate reconciled")
	}

	// Load the position (Master JD) + canonical candidate (for province).
	pos, err := pr.positions.FindByID(ctx, positionID)
	if err != nil {
		return fmt.Errorf("pipeline: load position: %w", err)
	}
	cand, err := pr.candidates.FindByID(ctx, canonicalID)
	if err != nil {
		return fmt.Errorf("pipeline: load candidate: %w", err)
	}
	jd := scoring.JD{
		Title:               pos.TitleTH,
		MinEducationLevel:   pos.MustHave.MinEducationLevel,
		MinExperienceMonths: pos.MustHave.MinExperienceMonths,
		Keywords:            pos.Keywords,
		Responsibilities:    pos.Responsibilities,
		Qualifications:      pos.Qualifications,
	}

	// Step 4 — Score (location signal first so it folds into the total).
	locationScore, err := pr.assigner.LocationScore(ctx, cand.Province, positionID)
	if err != nil {
		return fmt.Errorf("pipeline: location score: %w", err)
	}
	result, err := pr.scorer.Score(ctx, profile, jd, locationScore)
	if err != nil {
		return fmt.Errorf("pipeline: score: %w", err)
	}

	// Step 5 — Must-have gate.
	if !result.MustHavePassed {
		if err := pr.persistScore(ctx, appID, applications.StatusRejected, result); err != nil {
			return err
		}
		logger.Info().Int("score", result.Total).Msg("auto-rejected (must-have gate)")
		return nil
	}

	// Step 6 — Branch assignment.
	assignment, err := pr.assigner.Assign(ctx, cand.Province, positionID, pos.FormatTypes)
	if err != nil {
		return fmt.Errorf("pipeline: assign: %w", err)
	}
	if err := pr.apps.SetAssignment(ctx, appID, assignment.StoreNo, assignment.TalentPool); err != nil {
		return fmt.Errorf("pipeline: set assignment: %w", err)
	}
	if err := pr.persistScore(ctx, appID, applications.StatusScored, result); err != nil {
		return err
	}

	logger.Info().
		Int("score", result.Total).
		Bool("talent_pool", assignment.TalentPool).
		Str("subregion", assignment.Subregion).
		Msg("application scored and assigned")
	return nil
}

// persistScore maps a scoring.Result into the repository's pre-serialized Score.
func (pr *Processor) persistScore(ctx context.Context, appID uuid.UUID, status string, result scoring.Result) error {
	breakdownJSON, err := json.Marshal(result.Breakdown)
	if err != nil {
		return fmt.Errorf("pipeline: marshal breakdown: %w", err)
	}
	suggestedJSON, err := json.Marshal(result.SuggestedPositions)
	if err != nil {
		return fmt.Errorf("pipeline: marshal suggested: %w", err)
	}
	return pr.apps.SetScore(ctx, appID, applications.Score{
		Status:         status,
		MustHavePassed: result.MustHavePassed,
		Total:          float64(result.Total),
		BreakdownJSON:  breakdownJSON,
		Summary:        strings.Join(result.Strengths, "\n"),
		RedFlags:       strings.Join(result.RedFlags, "; "),
		SuggestedJSON:  suggestedJSON,
	})
}
