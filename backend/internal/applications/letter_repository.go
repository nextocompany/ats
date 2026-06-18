package applications

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/nexto/hr-ats/internal/letters"
)

// GatherLetterData assembles the renderer input for a letter, pulling the
// candidate/position/store context plus the type-specific block (the scheduled
// interview, or the offer). Returns ErrLetterPreconditions when the source data
// for the requested type is absent.
func (r *pgRepository) GatherLetterData(ctx context.Context, applicationID uuid.UUID, letterType string) (letters.LetterData, error) {
	var (
		name, positionTitle, storeName, status string
	)
	const q = `
		SELECT c.full_name,
		       COALESCE(NULLIF(p.title_en,''), p.title_th, ''),
		       COALESCE(s.store_name,''),
		       a.status
		FROM applications a
		JOIN candidates c ON c.id = a.candidate_id
		JOIN positions p ON p.id = a.position_id
		LEFT JOIN stores s ON s.store_no = a.assigned_store_id
		WHERE a.id = $1`
	if err := r.pool.QueryRow(ctx, q, applicationID).Scan(&name, &positionTitle, &storeName, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return letters.LetterData{}, ErrLetterPreconditions
		}
		return letters.LetterData{}, fmt.Errorf("applications: gather letter base: %w", err)
	}

	data := letters.LetterData{
		Type:          letterType,
		CandidateName: name,
		PositionTitle: positionTitle,
		StoreName:     storeName,
	}

	switch letterType {
	case LetterInterview:
		appt, err := r.FindAppointment(ctx, applicationID)
		if err != nil {
			return letters.LetterData{}, err
		}
		if appt == nil {
			return letters.LetterData{}, ErrLetterPreconditions
		}
		data.Interview = &letters.InterviewDetails{
			ScheduledAt: appt.ScheduledAt,
			DurationMin: appt.DurationMin,
			Mode:        appt.Mode,
			Location:    appt.LocationText,
			JoinURL:     appt.OnlineJoinURL,
		}
	case LetterOffer:
		offer, err := r.GetOfferByApplication(ctx, applicationID)
		if err != nil {
			return letters.LetterData{}, err
		}
		// An offer letter is only meaningful once an offer has been composed and
		// at least sent (a bare draft has no committed terms to put in writing).
		if offer == nil || offer.Status == OfferDraft {
			return letters.LetterData{}, ErrLetterPreconditions
		}
		var salary float64
		if offer.Salary != nil {
			salary = *offer.Salary
		}
		od := &letters.OfferDetails{Salary: salary, Terms: offer.Terms}
		if offer.StartDate != nil {
			od.StartDate = *offer.StartDate
		}
		data.Offer = od
	default:
		return letters.LetterData{}, fmt.Errorf("applications: unknown letter type %q", letterType)
	}
	return data, nil
}

// UpsertLetter stores (or replaces) the current letter record for an
// (application, type), returning the persisted row.
func (r *pgRepository) UpsertLetter(ctx context.Context, applicationID, createdBy uuid.UUID, letterType, blobURL string) (Letter, error) {
	const q = `
		INSERT INTO letters (application_id, type, blob_url, created_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (application_id, type)
		DO UPDATE SET blob_url = EXCLUDED.blob_url, created_by = EXCLUDED.created_by, created_at = NOW()
		RETURNING id, application_id, type, blob_url, created_at`
	var l Letter
	if err := r.pool.QueryRow(ctx, q, applicationID, letterType, blobURL, createdBy).
		Scan(&l.ID, &l.ApplicationID, &l.Type, &l.BlobURL, &l.CreatedAt); err != nil {
		return Letter{}, fmt.Errorf("applications: upsert letter: %w", err)
	}
	return l, nil
}

func (r *pgRepository) GetLettersByApplication(ctx context.Context, applicationID uuid.UUID) ([]Letter, error) {
	const q = `SELECT id, application_id, type, blob_url, created_at FROM letters WHERE application_id = $1 ORDER BY created_at DESC`
	return r.scanLetters(ctx, q, applicationID)
}

func (r *pgRepository) GetLetterByID(ctx context.Context, id uuid.UUID) (*Letter, error) {
	const q = `SELECT id, application_id, type, blob_url, created_at FROM letters WHERE id = $1`
	var l Letter
	err := r.pool.QueryRow(ctx, q, id).Scan(&l.ID, &l.ApplicationID, &l.Type, &l.BlobURL, &l.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("applications: get letter: %w", err)
	}
	return &l, nil
}

// ListLettersByAccount returns the letters across a member's applications.
func (r *pgRepository) ListLettersByAccount(ctx context.Context, accountID uuid.UUID) ([]Letter, error) {
	const q = `
		SELECT l.id, l.application_id, l.type, l.blob_url, l.created_at
		FROM letters l
		JOIN applications a ON a.id = l.application_id
		JOIN candidates c ON c.id = a.candidate_id
		WHERE c.account_id = $1
		ORDER BY l.created_at DESC`
	return r.scanLetters(ctx, q, accountID)
}

func (r *pgRepository) scanLetters(ctx context.Context, q string, arg uuid.UUID) ([]Letter, error) {
	rows, err := r.pool.Query(ctx, q, arg)
	if err != nil {
		return nil, fmt.Errorf("applications: list letters: %w", err)
	}
	defer rows.Close()
	out := make([]Letter, 0)
	for rows.Next() {
		var l Letter
		if err := rows.Scan(&l.ID, &l.ApplicationID, &l.Type, &l.BlobURL, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("applications: scan letter: %w", err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: iterate letters: %w", err)
	}
	return out, nil
}
