package pdpa

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// fallbackVersion is used only if the registry has no current row (e.g. the seed
// migration has not run). It matches the value the apps historically sent so
// behavior is unchanged rather than empty.
const fallbackVersion = "1.0"

// ConsentDocument is one localized privacy/consent notice from the registry.
type ConsentDocument struct {
	Version   string `json:"version"`
	Locale    string `json:"locale"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	IsCurrent bool   `json:"is_current"`
}

// CurrentVersion returns the version string of the current consent notice, or the
// fallback when the registry is empty. New consents stamp this so the recorded
// version always maps to a real registry document.
func (r *Repo) CurrentVersion(ctx context.Context) (string, error) {
	var v string
	err := r.pool.QueryRow(ctx,
		`SELECT version FROM consent_documents WHERE is_current ORDER BY effective_at DESC LIMIT 1`,
	).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return fallbackVersion, nil
	}
	if err != nil {
		return "", fmt.Errorf("pdpa: current version: %w", err)
	}
	return v, nil
}

// CurrentDocuments returns the current notice for a locale (falling back to 'th'
// when the requested locale has no current row).
func (r *Repo) CurrentDocuments(ctx context.Context, locale string) (ConsentDocument, error) {
	const q = `
		SELECT version, locale, title, body, is_current
		FROM consent_documents
		WHERE is_current AND locale = $1
		LIMIT 1`
	var d ConsentDocument
	err := r.pool.QueryRow(ctx, q, locale).Scan(&d.Version, &d.Locale, &d.Title, &d.Body, &d.IsCurrent)
	if errors.Is(err, pgx.ErrNoRows) && locale != "th" {
		return r.CurrentDocuments(ctx, "th")
	}
	if err != nil {
		return ConsentDocument{}, fmt.Errorf("pdpa: current document: %w", err)
	}
	return d, nil
}
