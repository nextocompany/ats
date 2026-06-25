package applications

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/rbac"
)

// Onboarding documents (Module-3 3.8). After offer accept advances an application
// to `hired`, the candidate uploads a checklist of required onboarding documents
// via the career-portal and HR reviews each one (approve/reject with a reason).
// The document lifecycle lives on the onboarding_documents table (migration
// 000025); the application funnel is unchanged — onboarding completion is derived
// (every required doc_type has an approved document).

// Known onboarding document types (mirror the migration CHECK constraint). The set
// of *required* types is config-driven (ONBOARDING_REQUIRED_DOCS) — a subset of
// these; military_certificate + name_change are known but optional by default.
const (
	DocIDCard               = "id_card"
	DocHouseRegistration    = "house_registration"
	DocEducationCertificate = "education_certificate"
	DocBankBook             = "bank_book"
	DocTaxDocument          = "tax_document"
	DocPhoto                = "photo"
	DocHealthCheck          = "health_check"
	DocMilitaryCertificate  = "military_certificate"
	DocNameChange           = "name_change"
)

var knownDocTypes = map[string]bool{
	DocIDCard: true, DocHouseRegistration: true, DocEducationCertificate: true,
	DocBankBook: true, DocTaxDocument: true, DocPhoto: true, DocHealthCheck: true,
	DocMilitaryCertificate: true, DocNameChange: true,
}

func validDocType(t string) bool { return knownDocTypes[t] }

// Onboarding document review states (onboarding_documents.status).
const (
	OnbPending  = "pending"
	OnbApproved = "approved"
	OnbRejected = "rejected"
)

// HR review decision verbs (review endpoint).
const (
	OnbDecisionApprove = "approve"
	OnbDecisionReject  = "reject"
)

func validOnbDecision(d string) bool {
	return d == OnbDecisionApprove || d == OnbDecisionReject
}

// canManageOnboarding may review documents — now resolved via dynamic RBAC
// (rbac.PermOnboardingWrite).
func canManageOnboarding(role string) bool { return rbac.Can(role, rbac.PermOnboardingWrite) }

// Upload constraints (mirror the candidateauth resume upload — 10MB + the same
// accepted content types).
const maxOnboardingBytes = 10 * 1024 * 1024

var onboardingContentTypes = map[string]string{
	"application/pdf": "pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	"image/jpeg": "image",
	"image/png":  "image",
}

// extForContentType maps an accepted content type to the blob-key file extension.
func extForContentType(ct string) string {
	switch ct {
	case "application/pdf":
		return ".pdf"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	default:
		return ""
	}
}

// Sentinel errors mapped to HTTP status by the handlers.
var (
	ErrOnboardingNoHiredApp  = errors.New("applications: no hired application for this account")
	ErrOnboardingDocNotFound = errors.New("applications: onboarding document not found")
	ErrOnboardingDocConflict = errors.New("applications: onboarding document could not be reviewed")
)

// OnboardingDocument is one uploaded document + its review state. BlobURL is
// internal (json:"-"); the API exposes a freshly-signed URL via OnboardingDocView.
type OnboardingDocument struct {
	ID            uuid.UUID  `json:"id"`
	ApplicationID uuid.UUID  `json:"application_id"`
	DocType       string     `json:"doc_type"`
	Status        string     `json:"status"`
	BlobURL       string     `json:"-"`
	FileName      string     `json:"file_name,omitempty"`
	FileType      string     `json:"file_type,omitempty"`
	ReviewReason  string     `json:"review_reason,omitempty"`
	ReviewedBy    *uuid.UUID `json:"-"`
	UploadedAt    time.Time  `json:"uploaded_at"`
	ReviewedAt    *time.Time `json:"reviewed_at"`
}

// OnboardingDocView is the API-facing projection: the document fields minus the
// blob handle, plus a freshly-signed download URL.
type OnboardingDocView struct {
	ID           uuid.UUID  `json:"id"`
	DocType      string     `json:"doc_type"`
	Status       string     `json:"status"`
	FileName     string     `json:"file_name,omitempty"`
	FileType     string     `json:"file_type,omitempty"`
	ReviewReason string     `json:"review_reason,omitempty"`
	UploadedAt   time.Time  `json:"uploaded_at"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
	URL          string     `json:"url,omitempty"`
}

// OnboardingStatus is the checklist + progress for one application's onboarding.
// Closed is the derived "case finalized" signal: onboarding is approve-complete AND
// the hire was pushed to PeopleSoft (ps_synced_at set). It is an HR/internal signal,
// populated only on HR reads (the candidate sees Complete).
type OnboardingStatus struct {
	ApplicationID uuid.UUID           `json:"application_id"`
	Required      []string            `json:"required"`
	Documents     []OnboardingDocView `json:"documents"`
	ApprovedCount int                 `json:"approved_count"`
	RequiredCount int                 `json:"required_count"`
	Complete      bool                `json:"complete"`
	Closed        bool                `json:"closed"`
}

// OnboardingReviewInput is the HR approve/reject payload.
type OnboardingReviewInput struct {
	Decision string `json:"decision"` // approve | reject
	Reason   string `json:"reason"`   // required when Decision == reject
}

// computeComplete reports how many of the required doc types have an approved
// document, and whether all of them do (the derived onboarding-complete signal).
func computeComplete(required []string, docs []OnboardingDocView) (approved int, complete bool) {
	byType := make(map[string]string, len(docs))
	for _, d := range docs {
		byType[d.DocType] = d.Status
	}
	for _, t := range required {
		if byType[t] == OnbApproved {
			approved++
		}
	}
	return approved, len(required) > 0 && approved == len(required)
}
