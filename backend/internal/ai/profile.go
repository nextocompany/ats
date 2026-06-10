// Package ai defines the OCR and CV-parsing seams. Provider choice (mock vs
// Azure) lives only in this package — nothing else imports Azure SDKs or
// endpoints directly.
package ai

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

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
	Company        string `json:"company"`
	Position       string `json:"position"`
	DurationMonths int    `json:"duration_months"`
	Description    string `json:"description"`
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
	Education       []Education  `json:"education"`
	Skills          []string     `json:"skills"`
	Languages       []Language   `json:"languages"`
	DesiredPosition string       `json:"desired_position"`
}

// looseInt coerces a JSON value an LLM may emit as a number, a quoted string
// ("25"), an empty string, or null into an int. Non-numeric values yield 0.
func looseInt(raw json.RawMessage) int {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if s == "" || s == "null" {
		return 0
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int(f)
	}
	return 0
}

// UnmarshalJSON tolerates age arriving as a string/empty/null (LLM output drift).
func (p *Personal) UnmarshalJSON(b []byte) error {
	type alias Personal
	aux := struct {
		Age json.RawMessage `json:"age"`
		*alias
	}{alias: (*alias)(p)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	p.Age = looseInt(aux.Age)
	return nil
}

// UnmarshalJSON tolerates duration_months arriving as a string/empty/null.
func (e *Experience) UnmarshalJSON(b []byte) error {
	type alias Experience
	aux := struct {
		DurationMonths json.RawMessage `json:"duration_months"`
		*alias
	}{alias: (*alias)(e)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	e.DurationMonths = looseInt(aux.DurationMonths)
	return nil
}

// UnmarshalJSON tolerates year arriving as a string/empty/null.
func (e *Education) UnmarshalJSON(b []byte) error {
	type alias Education
	aux := struct {
		Year json.RawMessage `json:"year"`
		*alias
	}{alias: (*alias)(e)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	e.Year = looseInt(aux.Year)
	return nil
}

// Validate enforces the minimum required shape before persistence.
func (p Profile) Validate() error {
	if p.Personal.Name == "" {
		return fmt.Errorf("ai: profile personal.name is required")
	}
	return nil
}
