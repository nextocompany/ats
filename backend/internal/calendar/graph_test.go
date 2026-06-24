package calendar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestGraph returns a graphProvider pointed at a test server with a plain HTTP
// client (no OAuth round-trip).
func newTestGraph(serverURL string) graphProvider {
	return graphProvider{http: http.DefaultClient, baseURL: serverURL, mailbox: "interviews@ert.test"}
}

func TestGraph_OnlineInterview_MintsMeetingAndBooksEvent(t *testing.T) {
	var onlineHit, eventHit bool
	var eventBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/onlineMeetings"):
			onlineHit = true
			_ = json.NewEncoder(w).Encode(map[string]any{"joinWebUrl": "https://teams.microsoft.com/l/meetup-join/REAL"})
		case strings.HasSuffix(r.URL.Path, "/events"):
			eventHit = true
			_ = json.NewDecoder(r.Body).Decode(&eventBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "evt-123"})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	g := newTestGraph(srv.URL)
	start := time.Date(2026, 7, 1, 3, 0, 0, 0, time.UTC)
	res, err := g.CreateInterview(context.Background(), Appointment{
		Subject:       "สัมภาษณ์งาน",
		Start:         start,
		End:           start.Add(time.Hour),
		Mode:          modeOnline,
		AttendeeEmail: "candidate@example.com",
		AttendeeName:  "สมชาย",
	})
	if err != nil {
		t.Fatalf("CreateInterview: %v", err)
	}
	if !onlineHit || !eventHit {
		t.Fatalf("expected both endpoints hit (online=%v event=%v)", onlineHit, eventHit)
	}
	if res.JoinURL != "https://teams.microsoft.com/l/meetup-join/REAL" {
		t.Fatalf("joinURL not propagated: %q", res.JoinURL)
	}
	if res.EventID != "evt-123" {
		t.Fatalf("eventID not propagated: %q", res.EventID)
	}
	// The event must carry the candidate as an attendee.
	attendees, _ := eventBody["attendees"].([]any)
	if len(attendees) != 1 {
		t.Fatalf("expected 1 attendee, got %d", len(attendees))
	}
}

func TestGraph_OnsiteInterview_SkipsOnlineMeeting(t *testing.T) {
	var onlineHit, eventHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/onlineMeetings") {
			onlineHit = true
		}
		if strings.HasSuffix(r.URL.Path, "/events") {
			eventHit = true
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "evt-onsite"})
		}
	}))
	defer srv.Close()

	g := newTestGraph(srv.URL)
	start := time.Date(2026, 7, 1, 3, 0, 0, 0, time.UTC)
	res, err := g.CreateInterview(context.Background(), Appointment{
		Subject: "สัมภาษณ์", Start: start, End: start.Add(time.Hour),
		Mode: modeOnsite, LocationText: "สำนักงานใหญ่ ชั้น 10", AttendeeEmail: "c@example.com",
	})
	if err != nil {
		t.Fatalf("CreateInterview: %v", err)
	}
	if onlineHit {
		t.Fatal("onsite interview must not mint a Teams meeting")
	}
	if !eventHit || res.EventID != "evt-onsite" || res.JoinURL != "" {
		t.Fatalf("unexpected onsite result: %+v (eventHit=%v)", res, eventHit)
	}
}

func TestMock_OnlineReturnsJoinURL(t *testing.T) {
	res, err := mockProvider{}.CreateInterview(context.Background(), Appointment{Mode: modeOnline})
	if err != nil || res.JoinURL == "" {
		t.Fatalf("mock online should return a join url, got %+v err=%v", res, err)
	}
}

// TestGraph_AddsInterviewerAttendee proves the HR interviewer is added as a second
// attendee on the event when InterviewerEmail is set, and that with no interviewer
// only the candidate is present.
func TestGraph_AddsInterviewerAttendee(t *testing.T) {
	capture := func(appt Appointment) []any {
		var eventBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/onlineMeetings"):
				_ = json.NewEncoder(w).Encode(map[string]any{"joinWebUrl": "https://t/x"})
			case strings.HasSuffix(r.URL.Path, "/events"):
				_ = json.NewDecoder(r.Body).Decode(&eventBody)
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "evt"})
			}
		}))
		defer srv.Close()
		if _, err := newTestGraph(srv.URL).CreateInterview(context.Background(), appt); err != nil {
			t.Fatalf("CreateInterview: %v", err)
		}
		att, _ := eventBody["attendees"].([]any)
		return att
	}

	base := Appointment{Mode: modeOnline, AttendeeEmail: "cand@x.com", AttendeeName: "C"}
	if got := capture(base); len(got) != 1 {
		t.Errorf("no interviewer: want 1 attendee, got %d", len(got))
	}
	withHR := base
	withHR.InterviewerEmail = "hr@x.com"
	withHR.InterviewerName = "HR"
	if got := capture(withHR); len(got) != 2 {
		t.Errorf("with interviewer: want 2 attendees, got %d", len(got))
	}
}
