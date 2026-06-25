package applications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const offerColumns = `id, application_id, status, salary, start_date, COALESCE(terms,''), benefits, counter_salary, COALESCE(negotiation_note,''), negotiation_round, sent_at, responded_at, expires_at, COALESCE(decline_reason,''), created_at`

func scanOffer(row pgx.Row) (Offer, error) {
	var o Offer
	var benefits []byte
	err := row.Scan(&o.ID, &o.ApplicationID, &o.Status, &o.Salary, &o.StartDate, &o.Terms,
		&benefits, &o.CounterSalary, &o.NegotiationNote, &o.NegotiationRound,
		&o.SentAt, &o.RespondedAt, &o.ExpiresAt, &o.DeclineReason, &o.CreatedAt)
	if err != nil {
		return o, err
	}
	if len(benefits) > 0 {
		if jerr := json.Unmarshal(benefits, &o.Benefits); jerr != nil {
			return o, fmt.Errorf("applications: unmarshal benefits: %w", jerr)
		}
	}
	return o, nil
}

// marshalBenefits encodes a structured benefits list for the JSONB column; an
// empty list stores SQL NULL so the column round-trips to nil (not "[]").
func marshalBenefits(b []Benefit) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("applications: marshal benefits: %w", err)
	}
	return raw, nil
}

// CreateOffer opens a draft offer for an application. The UNIQUE(application_id)
// constraint makes a second create return ErrOfferExists.
func (r *pgRepository) CreateOffer(ctx context.Context, applicationID, createdBy uuid.UUID, in OfferInput) (Offer, error) {
	benefits, err := marshalBenefits(in.Benefits)
	if err != nil {
		return Offer{}, err
	}
	const q = `
		INSERT INTO offers (application_id, status, salary, start_date, terms, benefits, expires_at, created_by)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7, $8)
		RETURNING ` + offerColumns
	o, err := scanOffer(r.pool.QueryRow(ctx, q, applicationID, OfferDraft, in.Salary, in.StartDate, in.Terms, benefits, in.ExpiresAt, createdBy))
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
	benefits, err := marshalBenefits(in.Benefits)
	if err != nil {
		return Offer{}, err
	}
	const q = `
		UPDATE offers SET salary = $2, start_date = $3, terms = NULLIF($4,''), benefits = $5, expires_at = $6, updated_at = NOW()
		WHERE application_id = $1 AND status = 'draft'
		RETURNING ` + offerColumns
	o, err := scanOffer(r.pool.QueryRow(ctx, q, applicationID, in.Salary, in.StartDate, in.Terms, benefits, in.ExpiresAt))
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

// NegotiateOffer records a candidate counter-offer and pauses the offer at
// 'negotiating' in one transaction. The offer must belong to accountID, be 'sent',
// not be expired, and have rounds left (negotiation_round < maxRounds). The
// application status is intentionally left at 'offer' so HR can revise & re-send;
// because the application row never changes, negotiation does not appear in the
// status-history timeline (by design).
func (r *pgRepository) NegotiateOffer(ctx context.Context, offerID, accountID uuid.UUID, counter *float64, note string, maxRounds int) (Offer, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Offer{}, fmt.Errorf("applications: begin negotiate offer: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		status    string
		round     int
		expiresAt *time.Time
		acctOwner *uuid.UUID
	)
	err = tx.QueryRow(ctx, `
		SELECT o.status, o.negotiation_round, o.expires_at, c.account_id
		FROM offers o
		JOIN applications a ON a.id = o.application_id
		JOIN candidates c ON c.id = a.candidate_id
		WHERE o.id = $1
		FOR UPDATE OF o`, offerID).Scan(&status, &round, &expiresAt, &acctOwner)
	if errors.Is(err, pgx.ErrNoRows) {
		return Offer{}, ErrOfferNotFound
	}
	if err != nil {
		return Offer{}, fmt.Errorf("applications: lock offer: %w", err)
	}
	// Account-scope check: a candidate may only negotiate their own offer.
	if acctOwner == nil || *acctOwner != accountID {
		return Offer{}, ErrOfferNotFound
	}
	if status != OfferSent {
		return Offer{}, ErrOfferConflict
	}
	if expiresAt != nil && time.Now().After(*expiresAt) {
		return Offer{}, ErrOfferConflict
	}
	if round >= maxRounds {
		return Offer{}, ErrNegotiationClosed
	}

	tag, err := tx.Exec(ctx, `
		UPDATE offers
		SET status = 'negotiating', counter_salary = $2, negotiation_note = NULLIF($3,''),
		    negotiation_round = negotiation_round + 1, responded_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'sent'`, offerID, counter, note)
	if err != nil {
		return Offer{}, fmt.Errorf("applications: negotiate offer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Offer{}, ErrOfferConflict
	}

	if err := tx.Commit(ctx); err != nil {
		return Offer{}, fmt.Errorf("applications: commit negotiate offer: %w", err)
	}
	got, err := r.GetOfferByID(ctx, offerID)
	if err != nil {
		return Offer{}, err
	}
	if got == nil {
		return Offer{}, fmt.Errorf("applications: offer vanished after negotiate")
	}
	return *got, nil
}

// ReopenOffer returns a negotiating offer to 'draft' so HR can revise it and
// re-send via the existing UpdateOffer/SendOffer path. sent_at/responded_at/
// expires_at are cleared — a stale past expiry would make the re-sent offer appear
// instantly expired (effectiveOfferStatus); HR sets a new deadline in the edit.
func (r *pgRepository) ReopenOffer(ctx context.Context, applicationID uuid.UUID) (Offer, error) {
	const q = `
		UPDATE offers SET status = 'draft', sent_at = NULL, responded_at = NULL, expires_at = NULL, updated_at = NOW()
		WHERE application_id = $1 AND status = 'negotiating'
		RETURNING ` + offerColumns
	o, err := scanOffer(r.pool.QueryRow(ctx, q, applicationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Offer{}, ErrOfferNotEditable
	}
	if err != nil {
		return Offer{}, fmt.Errorf("applications: reopen offer: %w", err)
	}
	return o, nil
}

// WithdrawOffer is the HR "end the negotiation / reject the candidate" path: it
// terminalizes the offer (declined) and rejects the application in one transaction
// so the two never diverge — a negotiating offer on a rejected application would
// keep showing the candidate a live counter. Mirrors RespondOffer's decline guard.
func (r *pgRepository) WithdrawOffer(ctx context.Context, applicationID uuid.UUID, reason string) (Offer, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Offer{}, fmt.Errorf("applications: begin withdraw offer: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE offers SET status = 'declined', decline_reason = $2, responded_at = NOW(), updated_at = NOW()
		WHERE application_id = $1 AND status IN ('sent','negotiating')`, applicationID, reason)
	if err != nil {
		return Offer{}, fmt.Errorf("applications: withdraw offer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Offer{}, ErrOfferConflict
	}
	tag, err = tx.Exec(ctx, `
		UPDATE applications SET status = $2, rejection_reason = $3, updated_at = NOW()
		WHERE id = $1 AND status = $4`,
		applicationID, StatusRejected, "HR ended the negotiation: "+reason, StatusOffer)
	if err != nil {
		return Offer{}, fmt.Errorf("applications: reject after withdraw: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Offer{}, ErrOfferConflict
	}

	if err := tx.Commit(ctx); err != nil {
		return Offer{}, fmt.Errorf("applications: commit withdraw offer: %w", err)
	}
	got, err := r.GetOfferByApplication(ctx, applicationID)
	if err != nil {
		return Offer{}, err
	}
	if got == nil {
		return Offer{}, ErrOfferNotFound
	}
	return *got, nil
}

// ListOffersByAccount returns the offers across a member's applications (active +
// decided), newest first, with position context for the portal list.
func (r *pgRepository) ListOffersByAccount(ctx context.Context, accountID uuid.UUID) ([]OfferView, error) {
	const q = `
		SELECT o.id, o.application_id, o.status, o.salary, o.start_date, COALESCE(o.terms,''),
		       o.benefits, o.counter_salary, COALESCE(o.negotiation_note,''), o.negotiation_round,
		       o.sent_at, o.responded_at, o.expires_at, COALESCE(o.decline_reason,''), o.created_at,
		       COALESCE(NULLIF(p.title_en,''), p.title_th, ''), a.assigned_store_id
		FROM offers o
		JOIN applications a ON a.id = o.application_id
		JOIN candidates c ON c.id = a.candidate_id
		JOIN positions p ON p.id = a.position_id
		-- 'expired' is a client-side computed state (effectiveOfferStatus), never
		-- stored; sent-past-expiry rows are included here via 'sent'. 'negotiating'
		-- is included so the candidate keeps seeing the live counter while HR revises.
		WHERE c.account_id = $1 AND o.status IN ('sent','negotiating','accepted','declined')
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
		var benefits []byte
		if err := rows.Scan(&v.ID, &v.ApplicationID, &v.Status, &v.Salary, &v.StartDate, &v.Terms,
			&benefits, &v.CounterSalary, &v.NegotiationNote, &v.NegotiationRound,
			&v.SentAt, &v.RespondedAt, &v.ExpiresAt, &v.DeclineReason, &v.CreatedAt,
			&v.PositionTitle, &v.StoreID); err != nil {
			return nil, fmt.Errorf("applications: scan offer view: %w", err)
		}
		if len(benefits) > 0 {
			if jerr := json.Unmarshal(benefits, &v.Benefits); jerr != nil {
				return nil, fmt.Errorf("applications: unmarshal benefits: %w", jerr)
			}
		}
		v.Status = effectiveOfferStatus(v.Offer, now)
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: iterate offers: %w", err)
	}
	return out, nil
}
