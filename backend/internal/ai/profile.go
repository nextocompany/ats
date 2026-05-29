// Package ai defines the OCR and CV-parsing seams. Provider choice (mock vs
// Azure) lives only in this package — nothing else imports Azure SDKs or
// endpoints directly.
package ai

import "fmt"

// Personal holds the candidate's personal fields (F02 schema).
type Personal struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
	Address string `json:"address"`
	Age     int    `json:"age"`
	IDCard  string `json:"id_card"`
}

// Experience is a single work-history entry.
type Experience struct {
	Company         string `json:"company"`
	Position        string `json:"position"`
	DurationMonths  int    `json:"duration_months"`
	Description     string `json:"description"`
}

// Education is a single education entry.
type Education struct {
	Degree      string `json:"degree"`
	Major       string `json:"major"`
	Institution string `json:"institution"`
	Year        int    `json:"year"`
}

// Language is a language proficiency entry.
type Language struct {
	Language string `json:"language"`
	Level    string `json:"level"`
}

// Profile is the structured CV produced by the parser (F02 schema).
type Profile struct {
	Personal        Personal     `json:"personal"`
	Experience      []Experience `json:"experience"`
	Education        []Education  `json:"education"`
	Skills          []string     `json:"skills"`
	Languages       []Language   `json:"languages"`
	DesiredPosition string       `json:"desired_position"`
}

// Validate enforces the minimum required shape before persistence.
func (p Profile) Validate() error {
	if p.Personal.Name == "" {
		return fmt.Errorf("ai: profile personal.name is required")
	}
	return nil
}
