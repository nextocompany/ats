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
	"github.com/nexto/hr-ats/internal/notify"
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

// accountProvisioner silently ensures an owning portal account for an intaken
// person, keyed by their parsed email (Phase 2 — unify candidates+members). The
// interface lives here so the pipeline doesn't import candidateauth's full
// surface. ok is false (nil error) when the email is empty/invalid. Default is a
// no-op; the worker injects the candidateauth-backed impl via SetAccountProvisioner.
type accountProvisioner interface {
	EnsureAccountByEmail(ctx context.Context, rawEmail string) (uuid.UUID, bool, error)
}

type noopAccountProvisioner struct{}

func (noopAccountProvisioner) EnsureAccountByEmail(context.Context, string) (uuid.UUID, bool, error) {
	return uuid.Nil, false, nil
}

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
	accounts   accountProvisioner
	hrNotify   hrNotify
	candNotify candNotify
}

// hrNotify bundles the optional HR-notification deps (best-effort, nil = no-op).
type hrNotify struct {
	notifier         notify.Notifier
	hr               applications.HRDirectory
	dashboardBaseURL string
	teamsEnabled     bool
}

// candNotify bundles the optional candidate-notification deps (best-effort, nil =
// no-op). Used to warn a candidate when their uploaded file is not a resume.
type candNotify struct {
	notifier      notify.Notifier
	portalBaseURL string
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
		indexer:  noopCandidateIndexer{},
		accounts: noopAccountProvisioner{},
	}
}

// SetAccountProvisioner injects silent at-intake account provisioning (Phase 2).
// No-op by default; the worker wires the candidateauth-backed implementation.
func (pr *Processor) SetAccountProvisioner(p accountProvisioner) {
	if p != nil {
		pr.accounts = p
	}
}

// SetIndexer injects a search indexer (no-op by default). Called by the worker
// when AI_SEARCH_PROVIDER=azure; ignored for nil so callers/tests stay simple.
func (pr *Processor) SetIndexer(idx CandidateIndexer) {
	if idx != nil {
		pr.indexer = idx
	}
}

// SetNotifier injects best-effort HR notifications fired when an application is
// scored + assigned to a store. No-op when notifier/hr are nil (tests/CI).
func (pr *Processor) SetNotifier(n notify.Notifier, hr applications.HRDirectory, dashboardBaseURL string, teamsEnabled bool) {
	pr.hrNotify = hrNotify{notifier: n, hr: hr, dashboardBaseURL: dashboardBaseURL, teamsEnabled: teamsEnabled}
}

// SetCandidateNotifier injects best-effort candidate notifications (LINE + email)
// fired when an uploaded file is detected as not a resume. No-op when the notifier
// is nil (tests/CI) or the candidate has no contact handle (e.g. bulk uploads).
func (pr *Processor) SetCandidateNotifier(n notify.Notifier, portalBaseURL string) {
	pr.candNotify = candNotify{notifier: n, portalBaseURL: portalBaseURL}
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

	canonicalID, err := pr.run(ctx, payload, appID, candID, positionID, logger)
	if err != nil {
		logger.Error().Err(err).Msg("pipeline failed")
		if serr := pr.apps.SetStatus(ctx, appID, applications.StatusFailed); serr != nil {
			logger.Error().Err(serr).Msg("failed to mark application failed")
		}
		return err
	}

	// Keep the search index fresh - best-effort. Index the CANONICAL candidate
	// (post-dedup), not the pre-dedup payload id: a deduped candidate is marked
	// is_duplicate_of and is excluded from the index projection, so indexing it
	// would no-op and leave the canonical doc stale (missing this application).
	// The canonical's searchable state (status/score/store) is now final; a stale
	// index must never fail the task.
	if err := pr.indexer.Index(ctx, canonicalID); err != nil {
		logger.Warn().Err(err).Msg("search index update failed (non-fatal)")
	}
	return nil
}

// run executes the pipeline and returns the CANONICAL candidate id (post-dedup)
// so the caller indexes the right candidate. The returned id is the pre-dedup
// candID until Step 3 resolves a canonical, so even an early error returns a
// sensible (if unused) id.
func (pr *Processor) run(ctx context.Context, p queue.ProcessApplicationPayload, appID, candID, positionID uuid.UUID, logger zerolog.Logger) (uuid.UUID, error) {
	canonicalID := candID

	// Step 1 — OCR.
	raw, err := pr.blob.Download(ctx, p.BlobName)
	if err != nil {
		return canonicalID, fmt.Errorf("pipeline: download: %w", err)
	}
	ocrRes, err := pr.ocr.Extract(ctx, raw, p.FileType)
	if err != nil {
		return canonicalID, fmt.Errorf("pipeline: ocr: %w", err)
	}
	needsReview := ocrRes.Confidence < ocrConfidenceThreshold
	if needsReview {
		logger.Warn().Float64("confidence", ocrRes.Confidence).Msg("low OCR confidence - flagging manual review")
	}
	ocrURL, err := pr.blob.Upload(ctx, fmt.Sprintf("ocr/%s/text.md", appID), []byte(ocrRes.Text), "text/markdown")
	if err != nil {
		return canonicalID, fmt.Errorf("pipeline: upload ocr text: %w", err)
	}

	// Step 2 — CV parse.
	profile, err := pr.parser.Parse(ctx, ocrRes.Text, p.PositionID)
	if err != nil {
		return canonicalID, fmt.Errorf("pipeline: parse: %w", err)
	}

	// Step 2a — not-a-resume gate. When the parser reports the document is not a
	// resume/CV, this is a terminal-but-recoverable outcome, NOT a transient
	// failure: mark the application invalid_resume, warn the candidate (best-effort),
	// and return nil so asynq does NOT retry (retrying a non-resume is pointless).
	// The candidate re-uploads a real CV to proceed.
	if !profile.IsResume {
		if serr := pr.apps.SetStatus(ctx, appID, applications.StatusInvalidResume); serr != nil {
			return canonicalID, fmt.Errorf("pipeline: set invalid_resume: %w", serr)
		}
		pr.notifyInvalidResume(ctx, appID, candID)
		logger.Info().Msg("uploaded file is not a resume - flagged invalid_resume")
		return canonicalID, nil
	}

	// Step 2b — name-mismatch gate (portal applies only). If the resume's name is
	// clearly a different person from the account holder, treat it like
	// invalid_resume: recoverable, warn the candidate, return nil (no retry). The
	// match is deliberately lenient (NameLooselyMatches) so Thai name variants /
	// nicknames / OCR slips never falsely reject a real applicant. Skipped for
	// accountless intakes (bulk/webhook) — no registered name to compare. Runs
	// BEFORE UpdateProfileFields overwrites the candidate's name with the parsed one.
	if nameTH, nameEN, hasAccount, nerr := pr.candidates.GetAccountMatchNames(ctx, candID); nerr != nil {
		log.Warn().Err(nerr).Str("candidate", candID.String()).Msg("name-mismatch gate: account name lookup failed (skipping check)")
	} else {
		resume := profile.Personal.Name
		hasName := nameTH != "" || nameEN != ""
		// A CV is written in one language, so accept the resume if it loosely
		// matches EITHER the Thai or the English name. NameMatchesAny skips empty
		// names (NameLooselyMatches treats an empty arg as a match, which would
		// otherwise auto-pass the gate and never flag a real mismatch).
		matched := dedup.NameMatchesAny(resume, nameTH, nameEN)
		if hasAccount && resume != "" && hasName && !matched {
			if serr := pr.apps.SetStatus(ctx, appID, applications.StatusNameMismatch); serr != nil {
				return canonicalID, fmt.Errorf("pipeline: set name_mismatch: %w", serr)
			}
			pr.notifyNameMismatch(ctx, appID, candID)
			logger.Info().Str("name_th", nameTH).Str("name_en", nameEN).Str("resume", resume).Msg("resume name does not match account - flagged name_mismatch")
			return canonicalID, nil
		}
	}

	if err := profile.Validate(); err != nil {
		return canonicalID, fmt.Errorf("pipeline: invalid profile: %w", err)
	}
	profileJSON, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return canonicalID, fmt.Errorf("pipeline: marshal profile: %w", err)
	}
	profileURL, err := pr.blob.Upload(ctx, fmt.Sprintf("profiles/%s/parsed_profile.json", appID), profileJSON, "application/json")
	if err != nil {
		return canonicalID, fmt.Errorf("pipeline: upload profile: %w", err)
	}
	if err := pr.candidates.UpdateProfileFields(ctx, candID, candidates.ProfileFields{
		FullName: profile.Personal.Name,
		Phone:    profile.Personal.Phone,
		Email:    profile.Personal.Email,
		Address:  profile.Personal.Address,
	}); err != nil {
		return canonicalID, fmt.Errorf("pipeline: update candidate: %w", err)
	}
	if err := pr.apps.SetParseResults(ctx, appID, applications.ParseResult{
		OCRTextBlobURL:       ocrURL,
		ParsedProfileBlobURL: profileURL,
		OCRConfidence:        ocrRes.Confidence,
		NeedsManualReview:    needsReview,
	}); err != nil {
		return canonicalID, fmt.Errorf("pipeline: set parse results: %w", err)
	}

	// Step 3 — Dedup. Reconcile the just-created candidate against existing ones.
	decision, err := pr.dedup.Reconcile(ctx, candID, profile.Personal.Name, profile.Personal.Phone, profile.Personal.Email, profile.Personal.IDCard)
	if err != nil {
		return canonicalID, fmt.Errorf("pipeline: dedup: %w", err)
	}
	canonicalID = decision.CanonicalID
	if decision.State != dedup.StateNone {
		if err := pr.apps.SetDedupState(ctx, appID, decision.State, decision.Confidence); err != nil {
			return canonicalID, fmt.Errorf("pipeline: set dedup state: %w", err)
		}
	}
	if canonicalID != candID {
		if err := pr.apps.SetCanonicalCandidate(ctx, appID, canonicalID); err != nil {
			return canonicalID, fmt.Errorf("pipeline: repoint candidate: %w", err)
		}
		logger.Info().Str("canonical", canonicalID.String()).Str("state", decision.State).Msg("duplicate reconciled")
	}

	// Load the position (Master JD) + canonical candidate (for province).
	pos, err := pr.positions.FindByID(ctx, positionID)
	if err != nil {
		return canonicalID, fmt.Errorf("pipeline: load position: %w", err)
	}
	cand, err := pr.candidates.FindByID(ctx, canonicalID)
	if err != nil {
		return canonicalID, fmt.Errorf("pipeline: load candidate: %w", err)
	}

	// Step 3b — silently ensure an owning portal account for the CANONICAL
	// candidate (Phase 2: keep the unified account-keyed Candidates list complete
	// at inflow, no backfill). Provision the canonical only — never candID — so a
	// duplicate intake row is never given its own account. Key the account by the
	// freshly PARSED email (profile.Personal.Email), not cand.Email: on an auto-merge
	// the parsed email lands on the duplicate row while the older canonical (e.g. an
	// accountless walk-in) may carry no email, so reading cand.Email would miss it.
	// Best-effort: a failure must not fail the task (no asynq retry storm); the
	// person is recovered on the next intake. No notification is ever sent here.
	if cand.AccountID == nil {
		if accountID, ok, perr := pr.accounts.EnsureAccountByEmail(ctx, profile.Personal.Email); perr != nil {
			logger.Warn().Err(perr).Str("canonical", canonicalID.String()).Msg("account provisioning failed (non-fatal)")
		} else if ok {
			if serr := pr.candidates.SetAccountID(ctx, canonicalID, accountID); serr != nil {
				logger.Warn().Err(serr).Str("canonical", canonicalID.String()).Msg("link candidate to account failed (non-fatal)")
			}
		}
	}

	jd := scoring.JD{
		Title:               pos.TitleTH,
		MinEducationLevel:   pos.MustHave.MinEducationLevel,
		MinExperienceMonths: pos.MustHave.MinExperienceMonths,
		Keywords:            pos.Keywords,
		Responsibilities:    pos.Responsibilities,
		Qualifications:      pos.Qualifications,
	}
	// Per-position screening weights (nil -> scorer applies DefaultWeights).
	if pos.ScoreWeights != nil {
		jd.Weights = *pos.ScoreWeights
	}

	// Step 4 — Score (location signal first so it folds into the total).
	locationScore, err := pr.assigner.LocationScore(ctx, cand.Province, positionID)
	if err != nil {
		return canonicalID, fmt.Errorf("pipeline: location score: %w", err)
	}
	result, err := pr.scorer.Score(ctx, profile, jd, locationScore)
	if err != nil {
		return canonicalID, fmt.Errorf("pipeline: score: %w", err)
	}

	// Step 5 — Must-have gate.
	if !result.MustHavePassed {
		if err := pr.persistScore(ctx, appID, applications.StatusRejected, result); err != nil {
			return canonicalID, err
		}
		pr.notifyCandidateStatus(ctx, appID, cand, applications.StatusRejected)
		logger.Info().Int("score", result.Total).Msg("auto-rejected (must-have gate)")
		return canonicalID, nil
	}

	// Step 6 — Branch assignment.
	assignment, err := pr.assigner.Assign(ctx, cand.Province, positionID, pos.FormatTypes)
	if err != nil {
		return canonicalID, fmt.Errorf("pipeline: assign: %w", err)
	}
	if err := pr.apps.SetAssignment(ctx, appID, assignment.StoreNo, assignment.TalentPool); err != nil {
		return canonicalID, fmt.Errorf("pipeline: set assignment: %w", err)
	}
	// Link the application to the matched vacancy so the requisition scope can
	// resolve it to the owning hiring manager (nil for talent-pool routing).
	if assignment.VacancyID != nil {
		if err := pr.apps.SetVacancy(ctx, appID, assignment.VacancyID); err != nil {
			return canonicalID, fmt.Errorf("pipeline: set vacancy: %w", err)
		}
	}
	if err := pr.persistScore(ctx, appID, applications.StatusScored, result); err != nil {
		return canonicalID, err
	}
	pr.notifyCandidateStatus(ctx, appID, cand, applications.StatusScored)

	logger.Info().
		Int("score", result.Total).
		Bool("talent_pool", assignment.TalentPool).
		Str("subregion", assignment.Subregion).
		Msg("application scored and assigned")

	// Step 7 — best-effort HR notification (email + Teams) for store-assigned hires.
	pr.notifyScored(ctx, appID, cand.FullName, pos.TitleTH, result.Total, assignment.StoreNo)
	// Step 7b — best-effort: ping the hiring manager who owns the matched
	// requisition (the position they opened) so they can review + approve.
	pr.notifyHiringManager(ctx, appID, assignment.VacancyID, cand.FullName, pos.TitleTH, result.Total)
	return canonicalID, nil
}

// notifyHiringManager pings the hiring manager who owns the matched requisition
// that a new candidate has entered the position they opened. No-op when deps are
// unset, the candidate routed to the talent pool (no vacancy), or the vacancy has
// no in-app hiring manager (e.g. PeopleSoft openings without an owner). Sent
// email-only: the recipient is one specific owner, and the shared Teams channel is
// already covered by the store-HR card above.
func (pr *Processor) notifyHiringManager(ctx context.Context, appID uuid.UUID, vacancyID *uuid.UUID, candName, positionTitle string, score int) {
	d := pr.hrNotify
	if d.notifier == nil || d.hr == nil || vacancyID == nil {
		return
	}
	email, _, err := d.hr.HiringManagerForVacancy(ctx, *vacancyID)
	if err != nil || email == "" {
		return // no owner to notify (or a read failure, logged at the repo layer)
	}
	dashURL := d.dashboardBaseURL + "/applications/" + appID.String()
	msgs := notify.NewCandidateHM([]string{email}, false, candName, positionTitle, score, dashURL)
	for _, m := range msgs {
		if err := d.notifier.Send(ctx, m); err != nil {
			log.Warn().Err(err).Str("application", appID.String()).Msg("hiring-manager notify: send failed (non-fatal)")
		}
	}
}

// notifyScored pings store HR that a new candidate was screened, scored, and
// assigned to their store. No-op when deps are unset or there is no assigned store
// (talent pool / unassigned candidates have no store HR to notify).
func (pr *Processor) notifyScored(ctx context.Context, appID uuid.UUID, candName, positionTitle string, score int, storeNo *int) {
	d := pr.hrNotify
	if d.notifier == nil || d.hr == nil || storeNo == nil {
		return
	}
	emails, err := d.hr.EmailsForStore(ctx, storeNo)
	if err != nil {
		return // logged at the repo layer; never fail the task
	}
	if len(emails) == 0 && !d.teamsEnabled {
		return
	}
	dashURL := d.dashboardBaseURL + "/applications/" + appID.String()
	msgs := notify.NewScoredHR(emails, d.teamsEnabled, candName, positionTitle, score, dashURL)
	for _, m := range msgs {
		if err := d.notifier.Send(ctx, m); err != nil {
			log.Warn().Err(err).Str("application", appID.String()).Msg("scored notify: send failed (non-fatal)")
		}
	}
}

// notifyInvalidResume warns the candidate (LINE + email, best-effort) that the
// file they uploaded is not a resume. Uses candID (dedup has not run yet, so there
// is no canonical). No-op when the notifier is unset or the candidate has no contact
// handle (bulk/walk-in uploads) — those surface to HR via the invalid_resume status.
func (pr *Processor) notifyInvalidResume(ctx context.Context, appID, candID uuid.UUID) {
	d := pr.candNotify
	if d.notifier == nil {
		return
	}
	cand, err := pr.candidates.FindByID(ctx, candID)
	if err != nil {
		log.Warn().Err(err).Str("candidate", candID.String()).Msg("invalid-resume notify: load candidate failed")
		return
	}
	token := pr.publicToken(ctx, appID)
	if msg := notify.InvalidResumeMessage(cand.LineUserID, cand.FullName, d.portalBaseURL, token); msg.Recipient != "" {
		if err := d.notifier.Send(ctx, msg); err != nil {
			log.Warn().Err(err).Str("application", appID.String()).Msg("invalid-resume notify: line send failed (non-fatal)")
		}
	}
	if em := notify.InvalidResumeEmailMessage(cand.Email, cand.FullName, d.portalBaseURL, token); em.Recipient != "" {
		if err := d.notifier.Send(ctx, em); err != nil {
			log.Warn().Err(err).Str("application", appID.String()).Msg("invalid-resume notify: email send failed (non-fatal)")
		}
	}
}

// notifyNameMismatch warns the candidate (LINE + email, best-effort) that the
// resume name does not match their account. Mirrors notifyInvalidResume.
func (pr *Processor) notifyNameMismatch(ctx context.Context, appID, candID uuid.UUID) {
	d := pr.candNotify
	if d.notifier == nil {
		return
	}
	cand, err := pr.candidates.FindByID(ctx, candID)
	if err != nil {
		log.Warn().Err(err).Str("candidate", candID.String()).Msg("name-mismatch notify: load candidate failed")
		return
	}
	token := pr.publicToken(ctx, appID)
	if msg := notify.NameMismatchMessage(cand.LineUserID, cand.FullName, d.portalBaseURL, token); msg.Recipient != "" {
		if err := d.notifier.Send(ctx, msg); err != nil {
			log.Warn().Err(err).Str("application", appID.String()).Msg("name-mismatch notify: line send failed (non-fatal)")
		}
	}
	if em := notify.NameMismatchEmailMessage(cand.Email, cand.FullName, d.portalBaseURL, token); em.Recipient != "" {
		if err := d.notifier.Send(ctx, em); err != nil {
			log.Warn().Err(err).Str("application", appID.String()).Msg("name-mismatch notify: email send failed (non-fatal)")
		}
	}
}

// notifyCandidateStatus sends the candidate a best-effort status update (LINE +
// email) for a pipeline auto-transition (scored / auto-rejected), so they hear
// about every meaningful change. No-op when the notifier is unset, the status has
// no candidate-facing copy, or the candidate has no contact handle.
func (pr *Processor) notifyCandidateStatus(ctx context.Context, appID uuid.UUID, cand *candidates.Candidate, status string) {
	d := pr.candNotify
	if d.notifier == nil || cand == nil {
		return
	}
	token := pr.publicToken(ctx, appID)
	if msg := notify.StatusMessage(cand.LineUserID, cand.FullName, status, d.portalBaseURL, token, appID); msg.Recipient != "" {
		if err := d.notifier.Send(ctx, msg); err != nil {
			log.Warn().Err(err).Str("status", status).Msg("candidate status notify: line send failed (non-fatal)")
		}
	}
	if em := notify.StatusEmailMessage(cand.Email, cand.FullName, status, d.portalBaseURL, token, appID); em.Recipient != "" {
		if err := d.notifier.Send(ctx, em); err != nil {
			log.Warn().Err(err).Str("status", status).Msg("candidate status notify: email send failed (non-fatal)")
		}
	}
}

// publicToken returns the application's status-page token (best-effort: a load
// failure yields "" → the notification falls back to a bare /status link).
func (pr *Processor) publicToken(ctx context.Context, appID uuid.UUID) string {
	app, err := pr.apps.FindByID(ctx, appID)
	if err != nil {
		log.Warn().Err(err).Str("application", appID.String()).Msg("notify: load public token failed (link will be generic)")
		return ""
	}
	return app.PublicToken
}

// persistScore maps a scoring.Result into the repository's pre-serialized Score.
func (pr *Processor) persistScore(ctx context.Context, appID uuid.UUID, status string, result scoring.Result) error {
	// Persist the raw per-dimension sub-scores plus the effective weights, so the
	// stored breakdown explains how the weighted Total was reached.
	breakdownJSON, err := json.Marshal(struct {
		scoring.Breakdown
		Weights scoring.Weights `json:"weights"`
	}{result.Breakdown, result.Weights})
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
