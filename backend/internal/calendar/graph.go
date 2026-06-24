package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2/clientcredentials"

	"github.com/nexto/hr-ats/pkg/config"
)

const (
	graphBase     = "https://graph.microsoft.com/v1.0"
	graphTimeout  = 30 * time.Second
	graphTokenURL = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
)

// graphProvider books interviews via Microsoft Graph using app-only (client
// credentials) auth on a single service mailbox.
type graphProvider struct {
	http    *http.Client
	baseURL string
	mailbox string
}

func newGraphProvider(cfg *config.Config) graphProvider {
	cc := &clientcredentials.Config{
		ClientID:     cfg.GraphClientID,
		ClientSecret: cfg.GraphClientSecret,
		TokenURL:     fmt.Sprintf(graphTokenURL, cfg.GraphTenantID),
		Scopes:       []string{"https://graph.microsoft.com/.default"},
	}
	hc := cc.Client(context.Background())
	hc.Timeout = graphTimeout
	return graphProvider{http: hc, baseURL: graphBase, mailbox: cfg.GraphOrganizerMailbox}
}

// CreateInterview mints the Teams meeting first (app-only `isOnlineMeeting=true`
// on an event is unreliable), then books the calendar event with the candidate as
// an attendee. The join URL is embedded in the event body. A Teams-mint failure is
// non-fatal: the event is still booked (empty JoinURL, error returned for logging).
func (g graphProvider) CreateInterview(ctx context.Context, a Appointment) (Result, error) {
	var joinURL string
	var mintErr error
	if a.Mode == modeOnline {
		joinURL, mintErr = g.createOnlineMeeting(ctx, a)
	}
	eventID, err := g.createEvent(ctx, a, joinURL)
	if err != nil {
		return Result{JoinURL: joinURL}, err
	}
	return Result{EventID: eventID, JoinURL: joinURL}, mintErr
}

// createOnlineMeeting mints a Teams meeting and returns its joinWebUrl. Requires
// the OnlineMeetings.ReadWrite application permission + a Teams Application Access
// Policy scoped to the service mailbox.
func (g graphProvider) createOnlineMeeting(ctx context.Context, a Appointment) (string, error) {
	payload := map[string]any{
		"startDateTime": a.Start.UTC().Format(time.RFC3339),
		"endDateTime":   a.End.UTC().Format(time.RFC3339),
		"subject":       a.Subject,
	}
	var out struct {
		JoinWebURL string `json:"joinWebUrl"`
	}
	url := fmt.Sprintf("%s/users/%s/onlineMeetings", g.baseURL, g.mailbox)
	if err := g.do(ctx, http.MethodPost, url, payload, &out); err != nil {
		return "", fmt.Errorf("calendar: create online meeting: %w", err)
	}
	return out.JoinWebURL, nil
}

// createEvent books a calendar event on the service mailbox with the candidate as
// a required attendee — Graph emails them the meeting invitation. Times are sent
// in UTC (timeZone "UTC") so there is no tzdata dependency in the container and
// every attendee's client localizes correctly.
func (g graphProvider) createEvent(ctx context.Context, a Appointment, joinURL string) (string, error) {
	body := a.BodyHTML
	location := a.LocationText
	if a.Mode == modeOnline {
		location = "Microsoft Teams Meeting"
		if joinURL != "" {
			body += fmt.Sprintf(`<p>Join online: <a href="%s">%s</a></p>`, joinURL, joinURL)
		}
	}
	attendees := []map[string]any{{
		"emailAddress": map[string]any{"address": a.AttendeeEmail, "name": a.AttendeeName},
		"type":         "required",
	}}
	// Add the scheduling HR user as a second required attendee so they get the
	// invite too (not just the candidate).
	if a.InterviewerEmail != "" {
		attendees = append(attendees, map[string]any{
			"emailAddress": map[string]any{"address": a.InterviewerEmail, "name": a.InterviewerName},
			"type":         "required",
		})
	}
	payload := map[string]any{
		"subject":   a.Subject,
		"body":      map[string]any{"contentType": "HTML", "content": body},
		"start":     map[string]any{"dateTime": a.Start.UTC().Format("2006-01-02T15:04:05"), "timeZone": "UTC"},
		"end":       map[string]any{"dateTime": a.End.UTC().Format("2006-01-02T15:04:05"), "timeZone": "UTC"},
		"location":  map[string]any{"displayName": location},
		"attendees": attendees,
	}
	var out struct {
		ID string `json:"id"`
	}
	url := fmt.Sprintf("%s/users/%s/events", g.baseURL, g.mailbox)
	if err := g.do(ctx, http.MethodPost, url, payload, &out); err != nil {
		return "", fmt.Errorf("calendar: create event: %w", err)
	}
	return out.ID, nil
}

// do issues a JSON request and decodes a JSON response, mirroring the external
// HTTP client pattern in internal/fit/azure.go.
func (g graphProvider) do(ctx context.Context, method, url string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decode: %w", err)
		}
	}
	return nil
}
