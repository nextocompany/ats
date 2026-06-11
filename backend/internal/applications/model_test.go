package applications

import (
	"encoding/json"
	"strings"
	"testing"
)

// The pipeline persists scoring.Breakdown into ai_score_breakdown as JSON; the
// detail read (FindByID) unmarshals that column back into ScoreBreakdown. This
// pins the key contract so a rename on either side fails loudly.
func TestScoreBreakdown_UnmarshalsPipelineJSON(t *testing.T) {
	// Exactly the shape scoring.Breakdown marshals to (keys + max ranges).
	raw := []byte(`{"experience":20,"skills":16,"education":8,"language":8,"location":15}`)

	var bd ScoreBreakdown
	if err := json.Unmarshal(raw, &bd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := bd
	want := ScoreBreakdown{Experience: 20, Skills: 16, Education: 8, Language: 8, Location: 15}
	if got != want {
		t.Errorf("breakdown = %+v, want %+v", got, want)
	}
}

// An unscored application has a NULL ai_score_breakdown; the detail panel keys
// off a nil pointer, so omitempty must drop the field entirely from JSON.
func TestApplication_OmitsEmptyExplainability(t *testing.T) {
	out, err := json.Marshal(Application{Status: StatusPending})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, absent := range []string{"ai_score_breakdown", "ai_summary", "ai_red_flags", "ai_suggested_positions"} {
		if strings.Contains(string(out), absent) {
			t.Errorf("expected %q to be omitted for an unscored application: %s", absent, string(out))
		}
	}
}
