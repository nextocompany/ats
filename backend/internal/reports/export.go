package reports

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
)

// Snapshot bundles all analytics for a period into one exportable record.
type Snapshot struct {
	Period  string   `json:"period"`
	Funnel  Funnel   `json:"funnel"`
	KPI     KPI      `json:"kpi"`
	Sources []Source `json:"sources"`
}

// Snapshot gathers funnel, KPI, and sources for the given period label, reusing
// the existing single-pass aggregations.
func (r *Repo) Snapshot(ctx context.Context, period string) (Snapshot, error) {
	funnel, err := r.Funnel(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	kpi, err := r.KPI(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	sources, err := r.Sources(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Period: period, Funnel: funnel, KPI: kpi, Sources: sources}, nil
}

// EncodeJSON renders the snapshot as indented JSON.
func EncodeJSON(s Snapshot) ([]byte, error) {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("reports: encode json: %w", err)
	}
	return b, nil
}

// EncodeCSV renders the snapshot as a single CSV with labelled sections
// (funnel, kpi, sources) so HR can open it directly in a spreadsheet.
func EncodeCSV(s Snapshot) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	rows := [][]string{
		{"section", "metric", "value"},
		{"funnel", "applied", strconv.Itoa(s.Funnel.Applied)},
		{"funnel", "passed_ai", strconv.Itoa(s.Funnel.PassedAI)},
		{"funnel", "reviewed", strconv.Itoa(s.Funnel.Reviewed)},
		{"funnel", "hired", strconv.Itoa(s.Funnel.Hired)},
		{"kpi", "applied", strconv.Itoa(s.KPI.Applied)},
		{"kpi", "passed", strconv.Itoa(s.KPI.Passed)},
		{"kpi", "onboarded", strconv.Itoa(s.KPI.Onboarded)},
		{"kpi", "waiting", strconv.Itoa(s.KPI.Waiting)},
	}
	for _, src := range s.Sources {
		rows = append(rows,
			[]string{"source:" + src.Channel, "applied", strconv.Itoa(src.Applied)},
			[]string{"source:" + src.Channel, "hired", strconv.Itoa(src.Hired)},
			[]string{"source:" + src.Channel, "conversion", strconv.FormatFloat(src.Conversion, 'f', 4, 64)},
		)
	}

	// WriteAll flushes internally; one error check covers write + flush.
	if err := w.WriteAll(rows); err != nil {
		return nil, fmt.Errorf("reports: encode csv: %w", err)
	}
	return buf.Bytes(), nil
}
