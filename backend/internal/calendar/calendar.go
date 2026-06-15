// Package calendar creates interview calendar events. For an online interview it
// mints a Microsoft Teams meeting and books a calendar event on a service mailbox
// with the candidate as an email attendee (Microsoft Graph emails them the invite).
// It follows the project's mock/real provider seam: mock (log-only) is the default
// so local/CI need no Microsoft credentials.
package calendar

import (
	"context"
	"time"

	"github.com/nexto/hr-ats/pkg/config"
)

// Appointment is the request to book an interview.
type Appointment struct {
	Subject       string
	BodyHTML      string
	Start         time.Time
	End           time.Time
	Mode          string // ModeOnsite | ModeOnline (mirrors applications.Mode*)
	LocationText  string // onsite address/room; online note
	AttendeeEmail string // candidate email (required for online)
	AttendeeName  string
}

// Result is what we persist on the appointment after booking.
type Result struct {
	EventID string // Graph event id (for a future cancel/reschedule)
	JoinURL string // Teams join link (online only)
}

const (
	modeOnsite = "onsite"
	modeOnline = "online"
)

// Provider books interview calendar events.
type Provider interface {
	CreateInterview(ctx context.Context, a Appointment) (Result, error)
}

// NewProvider returns the real Graph provider when GRAPH_PROVIDER=real, else the
// mock (log-only) provider.
func NewProvider(cfg *config.Config) Provider {
	if cfg.UsesRealGraph() {
		return newGraphProvider(cfg)
	}
	return mockProvider{}
}
