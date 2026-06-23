package applications

import (
	"context"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// maxBulkFiles caps a single bulk-upload request to 30 CVs. Each file is bounded
// by maxBulkResumeBytes (a generous 25MB — larger than any real CV, so size is
// effectively unconstrained for users, while a single pathological file still
// can't exhaust the container's memory).
const maxBulkFiles = 30

// maxBulkResumeBytes is the per-file ceiling for bulk upload (separate from the
// 10MB single-apply limit). 25MB comfortably covers scanned / image-heavy CVs.
const maxBulkResumeBytes = 25 * 1024 * 1024

// bulkIntaker is the narrow slice of the intake Service the bulk handler needs.
// *Service satisfies it; tests inject a fake.
type bulkIntaker interface {
	Intake(ctx context.Context, in IntakeInput) (IntakeResult, error)
}

// BulkHandler accepts many resume files for one position and creates one
// application (+ pipeline job) per file.
type BulkHandler struct {
	svc bulkIntaker
}

// NewBulkHandler builds the bulk-intake handler.
func NewBulkHandler(svc bulkIntaker) *BulkHandler { return &BulkHandler{svc: svc} }

// RegisterBulkRoutes mounts the bulk-intake endpoint.
func RegisterBulkRoutes(app *fiber.App, h *BulkHandler) {
	app.Post("/api/v1/applications/bulk-intake", h.BulkIntake)
}

type bulkCreated struct {
	Filename      string    `json:"filename"`
	ApplicationID uuid.UUID `json:"application_id"`
}

type bulkFailure struct {
	Filename string `json:"filename"`
	Error    string `json:"error"`
}

type bulkResult struct {
	Total       int           `json:"total"`
	Succeeded   int           `json:"succeeded"`
	FailedCount int           `json:"failed_count"`
	Created     []bulkCreated `json:"created"`
	Failed      []bulkFailure `json:"failed"`
}

// BulkIntake handles POST /api/v1/applications/bulk-intake (multipart: position_id
// + repeated resumes). One bad file never aborts the batch — failures are collected
// and returned alongside the successes. The candidate name is a placeholder
// (filename); the pipeline overwrites it from the parsed profile.
func (h *BulkHandler) BulkIntake(c *fiber.Ctx) error {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	if !rbac.Can(u.Role, rbac.PermBulkUpload) {
		return fiber.NewError(fiber.StatusForbidden, "insufficient role to upload CVs")
	}
	positionID, err := uuid.Parse(c.FormValue("position_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "valid position_id is required")
	}
	form, err := c.MultipartForm()
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid multipart form")
	}
	files := form.File["resumes"]
	if len(files) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "at least one resume file is required")
	}
	if len(files) > maxBulkFiles {
		return fiber.NewError(fiber.StatusBadRequest, "too many files in one request (max 30)")
	}
	sourceChannel := c.FormValue("source_channel")
	if sourceChannel == "" {
		sourceChannel = "bulk_upload"
	}

	result := bulkResult{Total: len(files), Created: []bulkCreated{}, Failed: []bulkFailure{}}
	for _, fh := range files {
		contentType := fh.Header.Get("Content-Type")
		fileType, ok := contentTypeToFileType[contentType]
		if !ok {
			result.Failed = append(result.Failed, bulkFailure{fh.Filename, "unsupported file type (allowed: pdf, docx, jpeg, png)"})
			continue
		}
		if fh.Size > maxBulkResumeBytes {
			result.Failed = append(result.Failed, bulkFailure{fh.Filename, "file exceeds 25MB limit"})
			continue
		}
		data, oerr := readMultipartFile(fh)
		if oerr != nil {
			result.Failed = append(result.Failed, bulkFailure{fh.Filename, "could not read uploaded file"})
			continue
		}
		res, ierr := h.svc.Intake(c.UserContext(), IntakeInput{
			CandidateName: placeholderName(fh.Filename),
			SourceChannel: sourceChannel,
			PositionID:    positionID,
			FileName:      fh.Filename,
			FileType:      fileType,
			ContentType:   contentType,
			FileBytes:     data,
		})
		if ierr != nil {
			result.Failed = append(result.Failed, bulkFailure{fh.Filename, "intake failed"})
			continue
		}
		result.Created = append(result.Created, bulkCreated{fh.Filename, res.ApplicationID})
	}
	result.Succeeded = len(result.Created)
	result.FailedCount = len(result.Failed)
	return httpx.OK(c, result)
}

// readMultipartFile opens + reads a multipart file header into memory.
func readMultipartFile(fh *multipart.FileHeader) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

// placeholderName derives a temporary candidate name from the filename (extension
// stripped). The pipeline replaces it with the parsed profile name. Falls back to a
// generic Thai label when the filename is empty.
func placeholderName(filename string) string {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	base = strings.TrimSpace(base)
	if base == "" {
		return "ผู้สมัคร"
	}
	return base
}
