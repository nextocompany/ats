package applications

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestScheduleResponse_FlattensAppointmentWithWarning proves the response shape:
// the embedded *Appointment fields stay top-level (so the existing client reads
// them unchanged) AND the optional warning serializes alongside. This guards the
// silent-drop failure mode — if Appointment ever grows a custom MarshalJSON, the
// promoted method would drop `warning` and this test would catch it.
func TestScheduleResponse_FlattensAppointmentWithWarning(t *testing.T) {
	appt := Appointment{
		ID:            uuid.New(),
		ApplicationID: uuid.New(),
		RoundNo:       1,
		Mode:          "online",
		// online_join_url intentionally empty (the failed-Teams case)
	}
	b, err := json.Marshal(scheduleResponse{Appointment: &appt, Warning: "ลิงก์ Teams ล้มเหลว"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	// Appointment fields must be top-level (flattened), not nested under a key.
	if !strings.Contains(s, `"application_id"`) || !strings.Contains(s, `"round_no"`) {
		t.Errorf("appointment fields not flattened: %s", s)
	}
	if strings.Contains(s, `"Appointment"`) {
		t.Errorf("appointment must be embedded (flattened), not nested: %s", s)
	}
	// The warning must be present.
	if !strings.Contains(s, `"warning":"ลิงก์ Teams ล้มเหลว"`) {
		t.Errorf("warning dropped from response: %s", s)
	}
}

// TestScheduleResponse_OmitsEmptyWarning proves the common (success) case carries
// no warning field, so an onsite or successfully-linked online interview looks
// exactly like the old response to the client.
func TestScheduleResponse_OmitsEmptyWarning(t *testing.T) {
	appt := Appointment{ID: uuid.New(), ApplicationID: uuid.New(), Mode: "onsite"}
	b, err := json.Marshal(scheduleResponse{Appointment: &appt})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "warning") {
		t.Errorf("empty warning must be omitted: %s", string(b))
	}
}
