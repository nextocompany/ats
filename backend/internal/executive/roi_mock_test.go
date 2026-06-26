package executive

import (
	"context"
	"testing"
)

// TestMockROI_SuccessReconciles asserts the demo's success table sums to its own
// headline hires for every dimension (mock runs with a nil pool → baked names).
func TestMockROI_SuccessReconciles(t *testing.T) {
	m := &mockService{pool: nil}
	for _, dim := range []string{"branch", "region", "position"} {
		for _, period := range []string{"month", "quarter", "year"} {
			v, err := m.ROI(context.Background(), ExecFilters{Period: period, Dimension: dim})
			if err != nil {
				t.Fatalf("mock ROI %s/%s: %v", dim, period, err)
			}
			var sum int
			for _, r := range v.Success {
				sum += r.Hires
			}
			if sum != v.Hires {
				t.Errorf("dim=%s period=%s: Σ success hires = %d, want headline %d", dim, period, sum, v.Hires)
			}
			if len(v.Success) == 0 {
				t.Errorf("dim=%s period=%s: success rows empty", dim, period)
			}
		}
	}
}

// TestMockROI_DeterministicAndEmptyCost verifies mock is repeatable and shows the
// ROI empty-state when no cost config exists (nil pool).
func TestMockROI_DeterministicAndEmptyCost(t *testing.T) {
	m := &mockService{pool: nil}
	a, _ := m.ROI(context.Background(), ExecFilters{Period: "quarter", Dimension: "branch"})
	b, _ := m.ROI(context.Background(), ExecFilters{Period: "quarter", Dimension: "branch"})
	if a.Hires != b.Hires || len(a.Success) != len(b.Success) {
		t.Errorf("mock ROI not deterministic: %d/%d vs %d/%d", a.Hires, len(a.Success), b.Hires, len(b.Success))
	}
	if a.CostConfigured || a.CostPerHire != 0 {
		t.Errorf("cost should be unset with nil pool, got configured=%v cph=%v", a.CostConfigured, a.CostPerHire)
	}
}
