// Package candidates owns candidate persistence and (in Sprint 2) dedup/merge.
package candidates

import (
	"time"

	"github.com/google/uuid"
)

// Candidate maps the candidates table (core columns used in Sprint 1).
type Candidate struct {
	ID            uuid.UUID  `json:"id"`
	FullName      string     `json:"full_name"`
	Phone         string     `json:"phone"`
	Email         string     `json:"email"`
	IDCard        string     `json:"id_card"`
	Address       string     `json:"address"`
	Province      string     `json:"province"`
	Subregion     string     `json:"subregion"`
	DateOfBirth   *time.Time `json:"date_of_birth"`
	SourceChannel string     `json:"source_channel"`
	Status        string     `json:"status"`
	// LineUserID is the verified LINE `sub` (from the LIFF id-token), captured at
	// apply time so real LINE push has a valid recipient. Empty for legacy/demo.
	LineUserID string    `json:"line_user_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ProfileFields are the parsed values written back after CV parsing.
type ProfileFields struct {
	FullName    string
	Phone       string
	Email       string
	Address     string
	Province    string
	DateOfBirth *time.Time
}
