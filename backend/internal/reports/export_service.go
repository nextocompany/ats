package reports

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/notify"
)

// exportSignedTTL must exceed the schedule interval so a delivered link stays
// valid until at least the next export.
const exportSignedTTL = 8 * 24 * time.Hour

// BlobStore is the subset of blob.Client the export service needs.
type BlobStore interface {
	Upload(ctx context.Context, name string, data []byte, contentType string) (string, error)
	SignedURLForStored(storedURL string, ttl time.Duration) (string, error)
}

// ExportService computes a report snapshot, stores CSV+JSON in blob, persists the
// export record, and delivers a signed link to the configured recipients.
type ExportService struct {
	repo       *Repo
	blob       BlobStore
	notifier   notify.Notifier
	recipients []string
}

// NewExportService builds the export service.
func NewExportService(repo *Repo, blob BlobStore, notifier notify.Notifier, recipients []string) *ExportService {
	return &ExportService{repo: repo, blob: blob, notifier: notifier, recipients: recipients}
}

// Export produces and delivers a report export for the given period. A delivery
// failure does not fail the export (it is recorded delivered=false); a blob
// failure does fail (so an asynq retry can re-attempt).
func (s *ExportService) Export(ctx context.Context, kind, period string) (Export, error) {
	snap, err := s.repo.Snapshot(ctx, period)
	if err != nil {
		return Export{}, fmt.Errorf("reports: snapshot: %w", err)
	}
	csvData, err := EncodeCSV(snap)
	if err != nil {
		return Export{}, fmt.Errorf("reports: encode csv: %w", err)
	}
	jsonData, err := EncodeJSON(snap)
	if err != nil {
		return Export{}, fmt.Errorf("reports: encode json: %w", err)
	}

	base := "reports/" + sanitize(period) + "-" + sanitize(kind)
	csvURL, err := s.blob.Upload(ctx, base+".csv", csvData, "text/csv")
	if err != nil {
		return Export{}, fmt.Errorf("reports: upload csv: %w", err)
	}
	jsonURL, err := s.blob.Upload(ctx, base+".json", jsonData, "application/json")
	if err != nil {
		return Export{}, fmt.Errorf("reports: upload json: %w", err)
	}

	// Persist BEFORE the side-effecting delivery so a record always exists for a
	// stored export. The record upserts, so this is safe under asynq retries.
	exp, err := s.repo.RecordExport(ctx, Export{
		Kind: kind, Period: period, CSVBlob: csvURL, JSONBlob: jsonURL, Delivered: false,
	})
	if err != nil {
		return Export{}, err
	}

	// Deliver best-effort; a delivery failure leaves delivered=false (no retry,
	// so the email never double-sends). MarkDelivered failure is logged, not fatal.
	if s.deliver(ctx, period, csvURL) {
		if err := s.repo.MarkDelivered(ctx, exp.ID); err != nil {
			log.Warn().Err(err).Str("period", period).Msg("reports: mark delivered failed")
		} else {
			exp.Delivered = true
		}
	}
	return exp, nil
}

// deliver sends the signed CSV link to each recipient. Returns true only if there
// was at least one recipient and all sends succeeded.
func (s *ExportService) deliver(ctx context.Context, period, csvURL string) bool {
	if len(s.recipients) == 0 {
		log.Info().Str("period", period).Msg("reports: no recipients configured — export stored, delivery skipped")
		return false
	}
	link, err := s.blob.SignedURLForStored(csvURL, exportSignedTTL)
	if err != nil {
		log.Warn().Err(err).Msg("reports: signed url failed — delivery skipped")
		return false
	}
	ok := true
	for _, to := range s.recipients {
		msg := notify.Message{
			Channel:   notify.ChannelEmail,
			Recipient: to,
			Subject:   "รายงานการสรรหา " + period,
			Body:      "ดาวน์โหลดรายงาน (CSV): " + link,
		}
		if err := s.notifier.Send(ctx, msg); err != nil {
			log.Warn().Err(err).Str("to", to).Msg("reports: export delivery failed")
			ok = false
		}
	}
	return ok
}

// sanitize keeps blob names filesystem/URL-safe.
func sanitize(s string) string {
	return strings.NewReplacer("/", "-", " ", "_", ":", "-").Replace(s)
}
