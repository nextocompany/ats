// Package dsar serves career-portal Data Subject Access Requests (Thai PDPA
// s.30 right of access + s.31 data portability): an authenticated candidate can
// export a complete, machine-readable copy of their own personal data. Erasure /
// rectification self-service builds on this package in a later slice.
package dsar

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service assembles a subject's export, strictly scoped to one portal account.
type Service struct{ pool *pgxpool.Pool }

// New builds the DSAR service.
func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Export is the full machine-readable copy of a subject's data. Blob CONTENTS are
// not inlined (binary); their metadata is, so the subject knows what is held and
// can request copies. Generated at export time.
type Export struct {
	Account      AccountExport       `json:"account"`
	Candidates   []CandidateExport   `json:"candidates"`
	Applications []ApplicationExport `json:"applications"`
	Interviews   []InterviewExport   `json:"interview_sessions"`
	Onboarding   []OnboardingExport  `json:"onboarding_documents"`
	Offers       []OfferExport       `json:"offers"`
	Letters      []LetterExport      `json:"letters"`
	Consents     []ConsentExport     `json:"consent_history"`
}

type AccountExport struct {
	ID          uuid.UUID `json:"id"`
	FullName    string    `json:"full_name"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	Province    string    `json:"province"`
	PDPAConsent bool      `json:"pdpa_consent"`
	PDPAVersion string    `json:"pdpa_version"`
	CreatedAt   time.Time `json:"created_at"`
}

type CandidateExport struct {
	ID        uuid.UUID `json:"id"`
	FullName  string    `json:"full_name"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	Province  string    `json:"province"`
	Source    string    `json:"source_channel"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type ApplicationExport struct {
	ID         uuid.UUID  `json:"id"`
	PositionID *uuid.UUID `json:"position_id"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
}

type InterviewExport struct {
	ApplicationID  uuid.UUID  `json:"application_id"`
	Status         string     `json:"status"`
	InterviewScore *float64   `json:"interview_score"`
	Recommendation *string    `json:"recommendation"`
	CompletedAt    *time.Time `json:"completed_at"`
}

type OnboardingExport struct {
	ApplicationID uuid.UUID `json:"application_id"`
	DocType       string    `json:"doc_type"`
	Status        string    `json:"status"`
	FileName      *string   `json:"file_name"`
	UploadedAt    time.Time `json:"uploaded_at"`
}

type OfferExport struct {
	ApplicationID uuid.UUID  `json:"application_id"`
	Status        string     `json:"status"`
	StartDate     *time.Time `json:"start_date"`
	CreatedAt     time.Time  `json:"created_at"`
}

type LetterExport struct {
	ApplicationID uuid.UUID `json:"application_id"`
	Type          string    `json:"type"`
	CreatedAt     time.Time `json:"created_at"`
}

type ConsentExport struct {
	Given     bool      `json:"consent_given"`
	Version   string    `json:"consent_version"`
	Source    string    `json:"source_channel"`
	CreatedAt time.Time `json:"created_at"`
}

// Export assembles the subject's complete record for one account. Every query is
// scoped to accountID (directly, or via candidates.account_id) so a subject can
// only ever read their own data.
func (s *Service) Export(ctx context.Context, accountID uuid.UUID) (Export, error) {
	var out Export

	if err := s.pool.QueryRow(ctx,
		`SELECT id, full_name, COALESCE(email,''), COALESCE(phone,''), COALESCE(province,''),
		        pdpa_consent, COALESCE(pdpa_version,''), created_at
		 FROM candidate_accounts WHERE id = $1`, accountID,
	).Scan(&out.Account.ID, &out.Account.FullName, &out.Account.Email, &out.Account.Phone,
		&out.Account.Province, &out.Account.PDPAConsent, &out.Account.PDPAVersion, &out.Account.CreatedAt); err != nil {
		return Export{}, fmt.Errorf("dsar: account: %w", err)
	}

	steps := []func(context.Context, uuid.UUID, *Export) error{
		s.candidates, s.applications, s.interviews, s.onboarding, s.offers, s.letters, s.consents,
	}
	for _, step := range steps {
		if err := step(ctx, accountID, &out); err != nil {
			return Export{}, err
		}
	}
	return out, nil
}

func (s *Service) candidates(ctx context.Context, accountID uuid.UUID, out *Export) error {
	rows, err := s.pool.Query(ctx,
		`SELECT id, full_name, COALESCE(phone,''), COALESCE(email,''), COALESCE(province,''),
		        COALESCE(source_channel,''), COALESCE(status,''), created_at
		 FROM candidates WHERE account_id = $1 ORDER BY created_at`, accountID)
	if err != nil {
		return fmt.Errorf("dsar: candidates: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c CandidateExport
		if err := rows.Scan(&c.ID, &c.FullName, &c.Phone, &c.Email, &c.Province, &c.Source, &c.Status, &c.CreatedAt); err != nil {
			return fmt.Errorf("dsar: scan candidate: %w", err)
		}
		out.Candidates = append(out.Candidates, c)
	}
	return rows.Err()
}

func (s *Service) applications(ctx context.Context, accountID uuid.UUID, out *Export) error {
	rows, err := s.pool.Query(ctx,
		`SELECT a.id, a.position_id, a.status, a.created_at
		 FROM applications a JOIN candidates c ON c.id = a.candidate_id
		 WHERE c.account_id = $1 ORDER BY a.created_at`, accountID)
	if err != nil {
		return fmt.Errorf("dsar: applications: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a ApplicationExport
		if err := rows.Scan(&a.ID, &a.PositionID, &a.Status, &a.CreatedAt); err != nil {
			return fmt.Errorf("dsar: scan application: %w", err)
		}
		out.Applications = append(out.Applications, a)
	}
	return rows.Err()
}

func (s *Service) interviews(ctx context.Context, accountID uuid.UUID, out *Export) error {
	rows, err := s.pool.Query(ctx,
		`SELECT i.application_id, i.status, i.interview_score, i.recommendation, i.completed_at
		 FROM interview_sessions i
		 JOIN applications a ON a.id = i.application_id
		 JOIN candidates c ON c.id = a.candidate_id
		 WHERE c.account_id = $1 ORDER BY i.invited_at`, accountID)
	if err != nil {
		return fmt.Errorf("dsar: interviews: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var i InterviewExport
		if err := rows.Scan(&i.ApplicationID, &i.Status, &i.InterviewScore, &i.Recommendation, &i.CompletedAt); err != nil {
			return fmt.Errorf("dsar: scan interview: %w", err)
		}
		out.Interviews = append(out.Interviews, i)
	}
	return rows.Err()
}

func (s *Service) onboarding(ctx context.Context, accountID uuid.UUID, out *Export) error {
	rows, err := s.pool.Query(ctx,
		`SELECT o.application_id, o.doc_type, o.status, o.file_name, o.uploaded_at
		 FROM onboarding_documents o
		 JOIN applications a ON a.id = o.application_id
		 JOIN candidates c ON c.id = a.candidate_id
		 WHERE c.account_id = $1 ORDER BY o.uploaded_at`, accountID)
	if err != nil {
		return fmt.Errorf("dsar: onboarding: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var o OnboardingExport
		if err := rows.Scan(&o.ApplicationID, &o.DocType, &o.Status, &o.FileName, &o.UploadedAt); err != nil {
			return fmt.Errorf("dsar: scan onboarding: %w", err)
		}
		out.Onboarding = append(out.Onboarding, o)
	}
	return rows.Err()
}

func (s *Service) offers(ctx context.Context, accountID uuid.UUID, out *Export) error {
	rows, err := s.pool.Query(ctx,
		`SELECT o.application_id, o.status, o.start_date, o.created_at
		 FROM offers o
		 JOIN applications a ON a.id = o.application_id
		 JOIN candidates c ON c.id = a.candidate_id
		 WHERE c.account_id = $1 ORDER BY o.created_at`, accountID)
	if err != nil {
		return fmt.Errorf("dsar: offers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var o OfferExport
		if err := rows.Scan(&o.ApplicationID, &o.Status, &o.StartDate, &o.CreatedAt); err != nil {
			return fmt.Errorf("dsar: scan offer: %w", err)
		}
		out.Offers = append(out.Offers, o)
	}
	return rows.Err()
}

func (s *Service) letters(ctx context.Context, accountID uuid.UUID, out *Export) error {
	rows, err := s.pool.Query(ctx,
		`SELECT l.application_id, l.type, l.created_at
		 FROM letters l
		 JOIN applications a ON a.id = l.application_id
		 JOIN candidates c ON c.id = a.candidate_id
		 WHERE c.account_id = $1 ORDER BY l.created_at`, accountID)
	if err != nil {
		return fmt.Errorf("dsar: letters: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var l LetterExport
		if err := rows.Scan(&l.ApplicationID, &l.Type, &l.CreatedAt); err != nil {
			return fmt.Errorf("dsar: scan letter: %w", err)
		}
		out.Letters = append(out.Letters, l)
	}
	return rows.Err()
}

// consents returns the unified consent trail for the subject: rows keyed directly
// on the account plus rows keyed on any of the subject's candidates.
func (s *Service) consents(ctx context.Context, accountID uuid.UUID, out *Export) error {
	rows, err := s.pool.Query(ctx,
		`SELECT consent_given, COALESCE(consent_version,''), COALESCE(source_channel,''), created_at
		 FROM pdpa_consents
		 WHERE account_id = $1
		    OR candidate_id IN (SELECT id FROM candidates WHERE account_id = $1)
		 ORDER BY created_at`, accountID)
	if err != nil {
		return fmt.Errorf("dsar: consents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ce ConsentExport
		if err := rows.Scan(&ce.Given, &ce.Version, &ce.Source, &ce.CreatedAt); err != nil {
			return fmt.Errorf("dsar: scan consent: %w", err)
		}
		out.Consents = append(out.Consents, ce)
	}
	return rows.Err()
}
