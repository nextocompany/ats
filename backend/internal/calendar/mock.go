package calendar

import (
	"context"

	"github.com/rs/zerolog/log"
)

// mockProvider logs the booking and returns a deterministic fake join URL for
// online interviews. Used by default so local/CI/tests need no Microsoft creds.
type mockProvider struct{}

func (mockProvider) CreateInterview(_ context.Context, a Appointment) (Result, error) {
	log.Info().
		Str("mode", a.Mode).
		Str("attendee", a.AttendeeEmail).
		Time("start", a.Start).
		Msg("calendar(mock): create interview")
	if a.Mode == modeOnline {
		return Result{EventID: "mock-event", JoinURL: "https://teams.microsoft.com/l/meetup-join/mock"}, nil
	}
	return Result{EventID: "mock-event"}, nil
}
