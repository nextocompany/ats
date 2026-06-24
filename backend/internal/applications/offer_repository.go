package applications

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const offerColumns = `id, application_id, status, salary, start_date, COALESCE(terms,''), sent_at, responded_at, expires_at, COALESCE(decline_reason,''), created_at`

func scanOffer(row pgx.Row) (Offer, error) {
	var o Offer
	err := row.Scan(&o.ID, &o.ApplicationID, &o.Status, &o.Salary, &o.StartDate, &o.Terms,
		&o.SentAt, &o.RespondedAt, &o.ExpiresAt, &o.DeclineReason, &o.CreatedAt)
	return o, err
}

// CreateOffer opens a draft offer for an application. The UNIQUE(application_id)
// constraint makes a second create return ErrOfferExists.
func (r *pgRepository) CreateOffer(ctx context.Context, applicationID, createdBy uuid.UUID, in OfferInput) (Offer, error) {
	const q = `
		INSERT INTO offers (application_id, status, salary, start_date, terms, expires_at, created_by)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7)
		RETURNING ` + offerColumns
	o, err := scanOffer(r.pool.QueryRow(ctx, q, applicationID, OfferDraft, in.Salary, in.StartDate, in.Terms, in.ExpiresAt, createdBy))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return Offer{}, ErrOfferExists
		}
		return Offer{}, fmt.Errorf("applications: create offer: %w", err)
	}
	return o, nil
}

// UpdateOffer edits a still-draft offer. A sent/decided offer is not editable.
func (r *pgRepository) UpdateOffer(ctx context.Context, applicationID uuid.UUID, in OfferInput) (Offer, error) {
	const q = `
		UPDATE offers SET salary = $2, start_date = $3, terms = NULLIF($4,''), expires_at = $5, updated_at = NOW()
		WHERE application_id = $1 AND status = 'draft'
		RETURNING ` + offerColumns
	o, err := scanOffer(r.pool.QueryRow(ctx, q, applicationID, in.Salary, in.StartDate, in.Terms, in.ExpiresAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return Offer{}, ErrOfferNotEditable
	}
	if err != nil {
		return Offer{}, fmt.Errorf("applications: update offer: %w", err)
	}
	return o, nil
}

func (r *pgRepository) GetOfferByApplication(ctx context.Context, applicationID uuid.UUID) (*Offer, error) {
	const q = `SELECT ` + offerColumns + ` FROM offers WHERE application_id = $1`
	o, err := scanOffer(r.pool.QueryRow(ctx, q, applicationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("applications: get offer: %w", err)
	}
	o.Status = effectiveOfferStatus(o, time.Now())
	return &o, nil
}

func (r *pgRepository) GetOfferByID(ctx context.Context, id uuid.UUID) (*Offer, error) {
	const q = `SELECT ` + offerColumns + ` FROM offers WHERE id = $1`
	o, err := scanOffer(r.pool.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("applications: get offer by id: %w", err)
	}
	o.Status = effectiveOfferStatus(o, time.Now())
	return &o, nil
}

// SendOffer transitions a draft offer to sent. Only a draft may be sent.
func (r *pgRepository) SendOffer(ctx context.Context, applicationID uuid.UUID) (Offer, error) {
	// salary/start_date guards are a backstop against a TOCTOU race where the offer
	// is edited incomplete between the handler's validation read and this send; the
	// handler also pre-validates for a friendly 400.
	const q = `
		UPDATE offers SET status = 'sent', sent_at = NOW(), updated_at = NOW()
		WHERE application_id = $1 AND status = 'draft' AND salary IS NOT NULL AND salary > 0 AND start_date IS NOT NULL
		RETURNING ` + offerColumns
	o, err := scanOffer(r.pool.QueryRow(ctx, q, applicationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Offer{}, ErrOfferConflict
	}
	if err != nil {
		return Offer{}, fmt.Errorf("applications: send offer: %w", err)
	}
	return o, nil
}

// RespondOffer records a candidate's accept/decline and flips the application in one
// transaction. The offer must belong to accountID, be 'sent', and not be expired.
// Accept → application 'hired'; decline → application 'rejected' (reason persisted).
func (r *pgRepository) RespondOffer(ctx context.Context, offerID, accountID uuid.UUID, accept bool, reason string) (Offer, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Offer{}, fmt.Errorf("applications: begin respond offer: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		appID     uuid.UUID
		status    string
		expiresAt *time.Time
		acctOwner *uuid.UUID
	)
	err = tx.QueryRow(ctx, `
		SELECT o.application_id, o.status, o.expires_at, c.account_id
		FROM offers o
		JOIN applications a ON a.id = o.application_id
		JOIN candidates c ON c.id = a.candidate_id
		WHERE o.id = $1
		FOR UPDATE OF o`, offerID).Scan(&appID, &status, &expiresAt, &acctOwner)
	if errors.Is(err, pgx.ErrNoRows) {
		return Offer{}, ErrOfferNotFound
	}
	if err != nil {
		return Offer{}, fmt.Errorf("applications: lock offer: %w", err)
	}
	// Account-scope check: a candidate may only respond to their own offer.
	if acctOwner == nil || *acctOwner != accountID {
		return Offer{}, ErrOfferNotFound
	}
	if status != OfferSent {
		return Offer{}, ErrOfferConflict
	}
	if expiresAt != nil && time.Now().After(*expiresAt) {
		return Offer{}, ErrOfferConflict
	}

	// The application row is guarded with `status = 'offer'` so a concurrent HR
	// action that already moved it off the offer stage (e.g. a reject PATCH) can't
	// be silently overwritten — 0 rows affected means the application moved on, so
	// we roll back and report a conflict (offer + application never diverge).
	if accept {
		if _, err := tx.Exec(ctx,
			`UPDATE offers SET status = 'accepted', responded_at = NOW(), updated_at = NOW() WHERE id = $1`, offerID); err != nil {
			return Offer{}, fmt.Errorf("applications: accept offer: %w", err)
		}
		tag, err := tx.Exec(ctx,
			`UPDATE applications SET status = $2, hired_at = NOW(), updated_at = NOW() WHERE id = $1 AND status = $3`,
			appID, StatusHired, StatusOffer)
		if err != nil {
			return Offer{}, fmt.Errorf("applications: set hired: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return Offer{}, ErrOfferConflict
		}
	} else {
		if _, err := tx.Exec(ctx,
			`UPDATE offers SET status = 'declined', decline_reason = $2, responded_at = NOW(), updated_at = NOW() WHERE id = $1`,
			offerID, reason); err != nil {
			return Offer{}, fmt.Errorf("applications: decline offer: %w", err)
		}
		tag, err := tx.Exec(ctx,
			`UPDATE applications SET status = $2, rejection_reason = $3, updated_at = NOW() WHERE id = $1 AND status = $4`,
			appID, StatusRejected, "Candidate declined the offer: "+reason, StatusOffer)
		if err != nil {
			return Offer{}, fmt.Errorf("applications: reject after decline: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return Offer{}, ErrOfferConflict
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Offer{}, fmt.Errorf("applications: commit respond offer: %w", err)
	}
	got, err := r.GetOfferByID(ctx, offerID)
	if err != nil {
		return Offer{}, err
	}
	if got == nil {
		return Offer{}, fmt.Errorf("applications: offer vanished after respond")
	}
	return *got, nil
}

// ListOffersByAccount returns the offers across a member's applications (active +
// decided), newest first, with position context for the portal list.
func (r *pgRepository) ListOffersByAccount(ctx context.Context, accountID uuid.UUID) ([]OfferView, error) {
	const q = `
		SELECT o.id, o.application_id, o.status, o.salary, o.start_date, COALESCE(o.terms,''),
		       o.sent_at, o.responded_at, o.expires_at, COALESCE(o.decline_reason,''), o.created_at,
		       COALESCE(NULLIF(p.title_en,''), p.title_th, ''), a.assigned_store_id
		FROM offers o
		JOIN applications a ON a.id = o.application_id
		JOIN candidates c ON c.id = a.candidate_id
		JOIN positions p ON p.id = a.position_id
		-- 'expired' is a client-side computed state (effectiveOfferStatus), never
		-- stored; sent-past-expiry rows are included here via 'sent'.
		WHERE c.account_id = $1 AND o.status IN ('sent','accepted','declined')
		ORDER BY o.created_at DESC`
	rows, err := r.pool.Query(ctx, q, accountID)
	if err != nil {
		return nil, fmt.Errorf("applications: list offers by account: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	out := make([]OfferView, 0)
	for rows.Next() {
		var v OfferView
		if err := rows.Scan(&v.ID, &v.ApplicationID, &v.Status, &v.Salary, &v.StartDate, &v.Terms,
			&v.SentAt, &v.RespondedAt, &v.ExpiresAt, &v.DeclineReason, &v.CreatedAt,
			&v.PositionTitle, &v.StoreID); err != nil {
			return nil, fmt.Errorf("applications: scan offer view: %w", err)
		}
		v.Status = effectiveOfferStatus(v.Offer, now)
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: iterate offers: %w", err)
	}
	return out, nil
}
