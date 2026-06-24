package scoring

import "testing"

func TestDefaultWeights_ValidAndSums100(t *testing.T) {
	w := DefaultWeights()
	if !w.Valid() {
		t.Fatalf("default weights must be valid, got %+v (sum %d)", w, w.Sum())
	}
	if w.Sum() != 100 {
		t.Fatalf("default weights must sum to 100, got %d", w.Sum())
	}
}

func TestWeights_Valid(t *testing.T) {
	cases := []struct {
		name string
		w    Weights
		want bool
	}{
		{"default", DefaultWeights(), true},
		{"sum 100", Weights{40, 20, 10, 10, 20}, true},
		{"sum 90", Weights{30, 20, 10, 10, 20}, false},
		{"sum 101", Weights{41, 20, 10, 10, 20}, false},
		{"negative", Weights{-10, 30, 30, 30, 20}, false},
		{"all on one", Weights{100, 0, 0, 0, 0}, true},
		{"zero value", Weights{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.w.Valid(); got != tc.want {
				t.Fatalf("Valid(%+v) = %v, want %v", tc.w, got, tc.want)
			}
		})
	}
}

func TestWeightedTotal_FullMarksIs100(t *testing.T) {
	full := Breakdown{Experience: CapExperience, Skills: CapSkills, Education: CapEducation, Language: CapLanguage, Location: CapLocation}
	if got := WeightedTotal(full, DefaultWeights()); got != 100 {
		t.Fatalf("max candidate with default weights should score 100, got %d", got)
	}
	if got := WeightedTotal(Breakdown{}, DefaultWeights()); got != 0 {
		t.Fatalf("empty breakdown should score 0, got %d", got)
	}
}

func TestWeightedTotal_DefaultMixed(t *testing.T) {
	// exp full (ratio 1), the other four at half their cap (ratio 0.5).
	bd := Breakdown{Experience: 30, Skills: 10, Education: 5, Language: 5, Location: 10}
	// 34*1 + (22+11+11+22)*0.5 = 34 + 33 = 67
	if got := WeightedTotal(bd, DefaultWeights()); got != 67 {
		t.Fatalf("expected 67, got %d", got)
	}
}

func TestWeightedTotal_CustomShiftsBlend(t *testing.T) {
	bd := Breakdown{Experience: 30, Skills: 0, Education: 0, Language: 0, Location: 10} // exp full, loc half
	// All weight on location: only location ratio (0.5) counts -> 50.
	allLoc := Weights{Experience: 0, Skills: 0, Education: 0, Language: 0, Location: 100}
	if got := WeightedTotal(bd, allLoc); got != 50 {
		t.Fatalf("all-location weight, loc at half -> 50, got %d", got)
	}
	// All weight on experience: exp full -> 100.
	allExp := Weights{Experience: 100}
	if got := WeightedTotal(bd, allExp); got != 100 {
		t.Fatalf("all-experience weight, exp full -> 100, got %d", got)
	}
}

func TestWeightedTotal_InvalidFallsBackToDefault(t *testing.T) {
	bd := Breakdown{Experience: 30, Skills: 20, Education: 10, Language: 10, Location: 20}
	full := WeightedTotal(bd, DefaultWeights())
	// zero-value (invalid) weights must behave as default, not 0.
	if got := WeightedTotal(bd, Weights{}); got != full {
		t.Fatalf("zero-value weights should fall back to default (%d), got %d", full, got)
	}
	// a sum!=100 set also falls back.
	if got := WeightedTotal(bd, Weights{30, 20, 10, 10, 20}); got != full {
		t.Fatalf("invalid (sum90) weights should fall back to default (%d), got %d", full, got)
	}
}

func TestWeightedTotal_RankingPreservedVsFlatSum(t *testing.T) {
	// Two candidates; default weights must keep the same ordering as the old flat sum
	// (monotonic rescale). a > b under flat sum -> a > b under weighted.
	a := Breakdown{Experience: 30, Skills: 15, Education: 10, Language: 10, Location: 20} // flat 85
	b := Breakdown{Experience: 20, Skills: 20, Education: 5, Language: 10, Location: 20}  // flat 75
	wa := WeightedTotal(a, DefaultWeights())
	wb := WeightedTotal(b, DefaultWeights())
	if wa <= wb {
		t.Fatalf("ranking not preserved: a=%d b=%d", wa, wb)
	}
}
