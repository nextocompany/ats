package reports

import (
	"encoding/json"
	"strings"
	"testing"
)

func sampleSnapshot() Snapshot {
	return Snapshot{
		Period:  "2026-W22",
		Funnel:  Funnel{Applied: 10, PassedAI: 6, Reviewed: 3, Hired: 1},
		KPI:     KPI{Applied: 10, Passed: 6, Onboarded: 1, Waiting: 4},
		Sources: []Source{{Channel: "career_portal", Applied: 8, Hired: 1, Conversion: 0.125}},
	}
}

func TestEncodeCSV_HasAllSections(t *testing.T) {
	out, err := EncodeCSV(sampleSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"section,metric,value", "funnel,applied,10", "kpi,onboarded,1", "source:career_portal,conversion,0.1250"} {
		if !strings.Contains(s, want) {
			t.Errorf("CSV missing %q\n%s", want, s)
		}
	}
}

func TestEncodeJSON_RoundTrips(t *testing.T) {
	out, err := EncodeJSON(sampleSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	var back Snapshot
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("json invalid: %v", err)
	}
	if back.Period != "2026-W22" || back.Funnel.Applied != 10 || len(back.Sources) != 1 {
		t.Fatalf("round-trip mismatch: %+v", back)
	}
}
