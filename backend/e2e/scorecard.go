// Package e2e — scorecard.go has NO build tag so the parse-accuracy scoring logic
// unit-tests in normal CI (no docker stack, no real CVs). The accuracy harness
// (accuracy_test.go, build tag `e2e`) feeds it real parsed profiles on staging.
package e2e

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/nexto/hr-ats/internal/ai"
)

// Expected is the gradable ground-truth subset of a CV (the *.expected.json shape).
// Only fields worth measuring are included; omit a field to skip grading it.
type Expected struct {
	Name                  string   `json:"name"`
	Phone                 string   `json:"phone"`
	Email                 string   `json:"email"`
	EducationLevel        string   `json:"education_level"`         // keyword, e.g. "bachelor" / "ตรี"
	TotalExperienceMonths int      `json:"total_experience_months"` // 0 = don't grade
	Skills                []string `json:"skills"`                  // recall over these
	Languages             []string `json:"languages"`               // recall over these
}

// FieldResult is one graded field's outcome. Score is 0..1 (exact fields are 0 or 1;
// recall fields are the fraction matched).
type FieldResult struct {
	Field string
	Score float64
	Got   string
	Want  string
}

// CVScore aggregates the graded fields for a single CV.
type CVScore struct {
	File          string
	OCRConfidence float64
	Fields        []FieldResult
}

// expMonthsTolerance is the ± window (months) within which total experience counts
// as correct (CVs rarely state exact months; ±1 year is a fair human-equivalent).
const expMonthsTolerance = 12

var nonDigit = regexp.MustCompile(`\D`)

func normText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

func digitsOnly(s string) string { return nonDigit.ReplaceAllString(s, "") }

func boolScore(ok bool) float64 {
	if ok {
		return 1
	}
	return 0
}

// totalExperienceMonths sums the parsed work-history durations.
func totalExperienceMonths(p ai.Profile) int {
	total := 0
	for _, e := range p.Experience {
		total += e.DurationMonths
	}
	return total
}

// educationContains reports whether any parsed education degree/level contains the
// expected level keyword (either direction, normalized) — tolerant of phrasing.
func educationContains(p ai.Profile, want string) bool {
	w := normText(want)
	if w == "" {
		return true
	}
	for _, e := range p.Education {
		d := normText(e.Degree)
		if d != "" && (strings.Contains(d, w) || strings.Contains(w, d)) {
			return true
		}
	}
	return false
}

// recall returns |expected ∩ got| / |expected| using normalized substring matching
// (a parsed skill counts if it contains, or is contained by, an expected skill).
func recall(got, want []string) (float64, int, int) {
	if len(want) == 0 {
		return 1, 0, 0
	}
	gn := make([]string, 0, len(got))
	for _, g := range got {
		if n := normText(g); n != "" {
			gn = append(gn, n)
		}
	}
	matched := 0
	for _, w := range want {
		wn := normText(w)
		if wn == "" {
			continue
		}
		for _, g := range gn {
			if strings.Contains(g, wn) || strings.Contains(wn, g) {
				matched++
				break
			}
		}
	}
	return float64(matched) / float64(len(want)), matched, len(want)
}

// Compare grades a parsed profile against the expected ground truth. Empty expected
// fields are skipped (not penalized).
func Compare(file string, ocrConfidence float64, p ai.Profile, exp Expected) CVScore {
	cv := CVScore{File: file, OCRConfidence: ocrConfidence}
	add := func(field string, score float64, got, want string) {
		cv.Fields = append(cv.Fields, FieldResult{Field: field, Score: score, Got: got, Want: want})
	}

	if exp.Name != "" {
		add("name", boolScore(normText(p.Personal.Name) == normText(exp.Name)), p.Personal.Name, exp.Name)
	}
	if exp.Email != "" {
		add("email", boolScore(strings.ToLower(strings.TrimSpace(p.Personal.Email)) == strings.ToLower(strings.TrimSpace(exp.Email))), p.Personal.Email, exp.Email)
	}
	if exp.Phone != "" {
		add("phone", boolScore(digitsOnly(p.Personal.Phone) == digitsOnly(exp.Phone)), p.Personal.Phone, exp.Phone)
	}
	if exp.EducationLevel != "" {
		add("education_level", boolScore(educationContains(p, exp.EducationLevel)), degrees(p), exp.EducationLevel)
	}
	if exp.TotalExperienceMonths > 0 {
		got := totalExperienceMonths(p)
		diff := got - exp.TotalExperienceMonths
		if diff < 0 {
			diff = -diff
		}
		add("experience_months", boolScore(diff <= expMonthsTolerance), fmt.Sprintf("%d", got), fmt.Sprintf("%d", exp.TotalExperienceMonths))
	}
	if len(exp.Skills) > 0 {
		r, m, n := recall(p.Skills, exp.Skills)
		add("skills_recall", r, fmt.Sprintf("%d/%d", m, n), strings.Join(exp.Skills, ","))
	}
	if len(exp.Languages) > 0 {
		got := make([]string, 0, len(p.Languages))
		for _, l := range p.Languages {
			got = append(got, l.Language)
		}
		r, m, n := recall(got, exp.Languages)
		add("languages_recall", r, fmt.Sprintf("%d/%d", m, n), strings.Join(exp.Languages, ","))
	}
	return cv
}

func degrees(p ai.Profile) string {
	out := make([]string, 0, len(p.Education))
	for _, e := range p.Education {
		if e.Degree != "" {
			out = append(out, e.Degree)
		}
	}
	return strings.Join(out, " | ")
}

// Aggregate computes per-field mean scores + the macro average across all CVs, plus
// OCR-confidence stats. Returned as a printable scorecard.
type Aggregate struct {
	Count        int
	PerField     map[string]float64 // field → mean score across CVs that graded it
	MacroAverage float64            // mean of per-field means
	OCRMean      float64
	OCRBelow070  int // count of CVs with OCR confidence < 0.70 (manual-review flag)
}

func AggregateScores(scores []CVScore) Aggregate {
	sum := map[string]float64{}
	cnt := map[string]int{}
	ocrSum, ocrBelow := 0.0, 0
	for _, cv := range scores {
		ocrSum += cv.OCRConfidence
		if cv.OCRConfidence < 0.70 {
			ocrBelow++
		}
		for _, f := range cv.Fields {
			sum[f.Field] += f.Score
			cnt[f.Field]++
		}
	}
	per := map[string]float64{}
	macroSum := 0.0
	for field, s := range sum {
		per[field] = s / float64(cnt[field])
		macroSum += per[field]
	}
	macro := 0.0
	if len(per) > 0 {
		macro = macroSum / float64(len(per))
	}
	ocrMean := 0.0
	if len(scores) > 0 {
		ocrMean = ocrSum / float64(len(scores))
	}
	return Aggregate{Count: len(scores), PerField: per, MacroAverage: macro, OCRMean: ocrMean, OCRBelow070: ocrBelow}
}

// Format renders a human-readable scorecard (printed by the accuracy harness).
func (a Aggregate) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Parse Accuracy Scorecard (%d CVs) ===\n", a.Count)
	fields := make([]string, 0, len(a.PerField))
	for f := range a.PerField {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	for _, f := range fields {
		fmt.Fprintf(&b, "  %-20s %5.1f%%\n", f, a.PerField[f]*100)
	}
	fmt.Fprintf(&b, "  %-20s %5.1f%%\n", "MACRO AVERAGE", a.MacroAverage*100)
	fmt.Fprintf(&b, "  OCR confidence mean  %5.2f  (below 0.70: %d/%d)\n", a.OCRMean, a.OCRBelow070, a.Count)
	return b.String()
}
