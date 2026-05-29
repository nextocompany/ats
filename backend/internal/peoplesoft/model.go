// Package peoplesoft integrates with PeopleSoft HCM (PRP §9): Direction A pulls
// vacancies via webhooks; Direction B pushes hired candidates via the
// Integration Broker REST API (mock by default, real behind config), with a
// CSV-to-Blob fallback when the broker is unavailable.
package peoplesoft

import "context"

// Candidate is the applicant payload sent to PeopleSoft.
type Candidate struct {
	FullName    string `json:"full_name"`
	IDCard      string `json:"id_card"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	DateOfBirth string `json:"date_of_birth"`
	Address     string `json:"address"`
}

// Applicant is the create_applicant request body (PRP §9 Direction B).
type Applicant struct {
	Action       string    `json:"action"`
	PSVacancyID  string    `json:"ps_vacancy_id"`
	Candidate    Candidate `json:"candidate"`
	SourceOfHire string    `json:"source_of_hire"`
	AppliedDate  string    `json:"applied_date"`
	HiredDate    string    `json:"hired_date"`
	AIScore      float64   `json:"ai_score"`
}

// BlobUploader is the subset of blob.Client used for the CSV fallback.
type BlobUploader interface {
	Upload(ctx context.Context, name string, data []byte, contentType string) (string, error)
}
