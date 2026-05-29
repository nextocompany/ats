package peoplesoft

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/applications"
	"github.com/nexto/hr-ats/internal/candidates"
)

const dateLayout = "2006-01-02"

// ApplicationStore is the narrow application-repository surface the sync needs.
type ApplicationStore interface {
	FindByID(ctx context.Context, id uuid.UUID) (*applications.Application, error)
	SetPSSynced(ctx context.Context, id uuid.UUID) error
}

// CandidateStore is the narrow candidate-repository surface the sync needs.
type CandidateStore interface {
	FindByID(ctx context.Context, id uuid.UUID) (*candidates.Candidate, error)
}

// Service orchestrates Direction B (hired → PeopleSoft) with a CSV fallback.
type Service struct {
	client    Client
	apps      ApplicationStore
	cands     CandidateStore
	blob      BlobUploader
	csvPrefix string
}

// NewService wires the PeopleSoft sync service. csvPrefix is the blob key prefix
// used for fallback exports (e.g. "ps-export").
func NewService(client Client, apps ApplicationStore, cands CandidateStore, blob BlobUploader, csvPrefix string) *Service {
	return &Service{client: client, apps: apps, cands: cands, blob: blob, csvPrefix: csvPrefix}
}

// SyncHired pushes a hired application to PeopleSoft. A PS failure never fails
// the hire: it writes a CSV fallback to Blob and leaves ps_synced_at unset.
func (s *Service) SyncHired(ctx context.Context, applicationID uuid.UUID) error {
	app, err := s.apps.FindByID(ctx, applicationID)
	if err != nil {
		return fmt.Errorf("peoplesoft: load application: %w", err)
	}
	cand, err := s.cands.FindByID(ctx, app.CandidateID)
	if err != nil {
		return fmt.Errorf("peoplesoft: load candidate: %w", err)
	}

	var score float64
	if app.AIScore != nil {
		score = *app.AIScore
	}
	applicant := Applicant{
		Action:      "create_applicant",
		PSVacancyID: "", // resolved from the assigned vacancy in a later iteration
		Candidate: Candidate{
			FullName: cand.FullName,
			IDCard:   cand.IDCard,
			Phone:    cand.Phone,
			Email:    cand.Email,
			Address:  cand.Address,
		},
		SourceOfHire: cand.SourceChannel,
		AppliedDate:  app.CreatedAt.Format(dateLayout),
		HiredDate:    time.Now().Format(dateLayout),
		AIScore:      score,
	}
	if cand.DateOfBirth != nil {
		applicant.Candidate.DateOfBirth = cand.DateOfBirth.Format(dateLayout)
	}

	if err := s.client.SyncHired(ctx, applicant); err != nil {
		log.Warn().Err(err).Str("application_id", applicationID.String()).Msg("peoplesoft sync failed — writing CSV fallback")
		if ferr := s.writeFallback(ctx, applicationID, applicant); ferr != nil {
			return fmt.Errorf("peoplesoft: sync and fallback both failed: %w", ferr)
		}
		return nil // hire preserved; ps_synced_at intentionally left unset
	}

	if err := s.apps.SetPSSynced(ctx, applicationID); err != nil {
		return fmt.Errorf("peoplesoft: mark synced: %w", err)
	}
	return nil
}

func (s *Service) writeFallback(ctx context.Context, appID uuid.UUID, a Applicant) error {
	var b strings.Builder
	b.WriteString("application_id,full_name,id_card,phone,email,source_of_hire,applied_date,hired_date,ai_score\n")
	b.WriteString(strings.Join([]string{
		appID.String(), csvEscape(a.Candidate.FullName), a.Candidate.IDCard, a.Candidate.Phone,
		a.Candidate.Email, a.SourceOfHire, a.AppliedDate, a.HiredDate, strconv.FormatFloat(a.AIScore, 'f', 2, 64),
	}, ","))
	b.WriteString("\n")

	name := fmt.Sprintf("%s/hired-%s.csv", strings.Trim(s.csvPrefix, "/"), appID)
	if _, err := s.blob.Upload(ctx, name, []byte(b.String()), "text/csv"); err != nil {
		return fmt.Errorf("peoplesoft: write fallback csv: %w", err)
	}
	log.Info().Str("blob", name).Msg("peoplesoft CSV fallback written")
	return nil
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
