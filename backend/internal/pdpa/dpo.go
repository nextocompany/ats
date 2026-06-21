package pdpa

import (
	"context"
	"fmt"
)

// DPOOfficer is one published Data Protection Officer (PDPA s.41). The set is
// derived from accounts flagged is_dpo, not a static config value, so the DPO
// contact stays correct as designations change.
type DPOOfficer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// DPODirectory is the published DPO block: the controller (company) plus every
// active officer. Officers is never null (empty slice) so clients can render a
// "not configured" state without a null check.
type DPODirectory struct {
	Company  string       `json:"company"`
	Officers []DPOOfficer `json:"officers"`
}

// ListDPOOfficers returns the active accounts designated as DPOs, ordered by name.
func (r *Repo) ListDPOOfficers(ctx context.Context) ([]DPOOfficer, error) {
	const q = `SELECT COALESCE(full_name, ''), email, COALESCE(phone, '')
		FROM users
		WHERE is_dpo = TRUE AND is_active = TRUE
		ORDER BY full_name, email`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("pdpa: list dpo officers: %w", err)
	}
	defer rows.Close()
	officers := []DPOOfficer{}
	for rows.Next() {
		var o DPOOfficer
		if err := rows.Scan(&o.Name, &o.Email, &o.Phone); err != nil {
			return nil, fmt.Errorf("pdpa: scan dpo officer: %w", err)
		}
		officers = append(officers, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pdpa: list dpo officers: %w", err)
	}
	return officers, nil
}
