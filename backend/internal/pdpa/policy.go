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

// canonicalLocale is the source of version truth: the recorded consent version is
// always read from this locale's current document so that, even if locales are
// promoted out of step, a single version is stamped. Operators must promote all
// locales of a new version together (the rendered notice locale should match).
const canonicalLocale = "th"

// ErrNoCurrentDoc signals the registry has no current document for any locale (a
// seeding/operational error, surfaced as 5xx rather than a client 404).
var ErrNoCurrentDoc = errors.New("pdpa: no current consent document")

// ConsentDocument is one localized privacy/consent notice from the registry.
type ConsentDocument struct {
	Version   string `json:"version"`
	Locale    string `json:"locale"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	IsCurrent bool   `json:"is_current"`
}

// CurrentVersion returns the version string of the current consent notice, read
// from the canonical locale so the stamped version is deterministic even if
// locales are mid-promotion. Falls back to "1.0" when the registry is empty.
func (r *Repo) CurrentVersion(ctx context.Context) (string, error) {
	var v string
	err := r.pool.QueryRow(ctx,
		`SELECT version FROM consent_documents WHERE is_current AND locale = $1 LIMIT 1`,
		canonicalLocale,
	).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		// No canonical-locale current row: fall back to any current row, then to
		// the historical default, so a recorded consent is never stamped empty.
		err = r.pool.QueryRow(ctx,
			`SELECT version FROM consent_documents WHERE is_current ORDER BY effective_at DESC LIMIT 1`,
		).Scan(&v)
		if errors.Is(err, pgx.ErrNoRows) {
			return fallbackVersion, nil
		}
	}
	if err != nil {
		return "", fmt.Errorf("pdpa: current version: %w", err)
	}
	return v, nil
}

// CurrentDocuments returns the current notice for a locale, falling back to any
// other current locale when the requested one has no current row. Returns
// ErrNoCurrentDoc when the registry has no current document at all.
func (r *Repo) CurrentDocuments(ctx context.Context, locale string) (ConsentDocument, error) {
	const q = `
		SELECT version, locale, title, body, is_current
		FROM consent_documents
		WHERE is_current AND locale = $1
		LIMIT 1`
	var d ConsentDocument
	err := r.pool.QueryRow(ctx, q, locale).Scan(&d.Version, &d.Locale, &d.Title, &d.Body, &d.IsCurrent)
	if errors.Is(err, pgx.ErrNoRows) {
		// Fall back to any current document (e.g. the requested locale has not been
		// translated for this version yet) rather than 404-ing the consent step.
		const anyQ = `
			SELECT version, locale, title, body, is_current
			FROM consent_documents
			WHERE is_current
			ORDER BY (locale = $1) DESC, locale
			LIMIT 1`
		err = r.pool.QueryRow(ctx, anyQ, canonicalLocale).Scan(&d.Version, &d.Locale, &d.Title, &d.Body, &d.IsCurrent)
		if errors.Is(err, pgx.ErrNoRows) {
			return ConsentDocument{}, ErrNoCurrentDoc
		}
	}
	if err != nil {
		return ConsentDocument{}, fmt.Errorf("pdpa: current document: %w", err)
	}
	return d, nil
}
