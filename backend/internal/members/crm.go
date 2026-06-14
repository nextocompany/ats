package members

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// AddNote appends an HR note to a member. A missing member surfaces as ErrNotFound
// (the member_notes.account_id FK rejects the insert).
//
// Notes/tags remain operable on an anonymized member by design: they are internal
// HR records that don't reproduce the erased PII (the account row survives
// anonymization as a redacted shell), so retaining and annotating them supports
// legitimate HR record-keeping without re-introducing personal data.
func (r *pgRepository) AddNote(ctx context.Context, id uuid.UUID, author, body string) (*Note, error) {
	const q = `
		INSERT INTO member_notes (account_id, author_email, body)
		VALUES ($1, $2, $3)
		RETURNING id, author_email, body, created_at`
	var n Note
	err := r.pool.QueryRow(ctx, q, id, author, body).Scan(&n.ID, &n.AuthorEmail, &n.Body, &n.CreatedAt)
	if isForeignKey(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("members: add note: %w", err)
	}
	return &n, nil
}

// ListNotes returns a member's notes newest-first.
func (r *pgRepository) ListNotes(ctx context.Context, id uuid.UUID) ([]Note, error) {
	const q = `
		SELECT id, author_email, body, created_at
		FROM member_notes WHERE account_id = $1
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("members: list notes: %w", err)
	}
	defer rows.Close()

	var out []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.AuthorEmail, &n.Body, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("members: scan note: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// AddTag attaches a tag to a member. Idempotent (ON CONFLICT DO NOTHING) so
// re-tagging is a no-op. A missing member surfaces as ErrNotFound via the FK.
func (r *pgRepository) AddTag(ctx context.Context, id uuid.UUID, tag string) error {
	const q = `INSERT INTO member_tags (account_id, tag) VALUES ($1, $2) ON CONFLICT (account_id, tag) DO NOTHING`
	_, err := r.pool.Exec(ctx, q, id, tag)
	if isForeignKey(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("members: add tag: %w", err)
	}
	return nil
}

// RemoveTag detaches a tag (no-op if absent).
func (r *pgRepository) RemoveTag(ctx context.Context, id uuid.UUID, tag string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM member_tags WHERE account_id = $1 AND tag = $2`, id, tag); err != nil {
		return fmt.Errorf("members: remove tag: %w", err)
	}
	return nil
}

// ListTags returns a member's tags sorted alphabetically.
func (r *pgRepository) ListTags(ctx context.Context, id uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT tag FROM member_tags WHERE account_id = $1 ORDER BY tag`, id)
	if err != nil {
		return nil, fmt.Errorf("members: list tags: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("members: scan tag: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
