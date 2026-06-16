package e2e

import (
	"math"
	"testing"

	"github.com/nexto/hr-ats/internal/ai"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func sampleProfile() ai.Profile {
	return ai.Profile{
		Personal: ai.Personal{Name: "สมชาย ใจดี", Phone: "08-1234-5678", Email: "Somchai@Example.com"},
		Experience: []ai.Experience{
			{DurationMonths: 24}, {DurationMonths: 12},
		},
		Education: []ai.Education{{Degree: "ปริญญาตรี (Bachelor)"}},
		Skills:    []string{"Golang", "PostgreSQL", "Docker"},
		Languages: []ai.Language{{Language: "Thai"}, {Language: "English"}},
	}
}

func TestCompare_ExactFields(t *testing.T) {
	exp := Expected{
		Name:  "สมชาย ใจดี",
		Phone: "0812345678",          // different formatting → digits-only match
		Email: "somchai@example.com", // case-insensitive match
	}
	cv := Compare("a.pdf", 0.9, sampleProfile(), exp)
	for _, f := range cv.Fields {
		if f.Score != 1 {
			t.Fatalf("field %s expected score 1, got %v (got=%q want=%q)", f.Field, f.Score, f.Got, f.Want)
		}
	}
}

func TestCompare_PhoneEmailNormalization(t *testing.T) {
	p := ai.Profile{Personal: ai.Personal{Phone: "0812345678", Email: "a@b.com"}}
	cv := Compare("x", 1, p, Expected{Phone: "081-234-5678", Email: "A@B.COM"})
	if len(cv.Fields) != 2 || cv.Fields[0].Score != 1 || cv.Fields[1].Score != 1 {
		t.Fatalf("phone/email normalization failed: %+v", cv.Fields)
	}
}

func TestCompare_ExperienceTolerance(t *testing.T) {
	p := ai.Profile{Experience: []ai.Experience{{DurationMonths: 30}}}
	// expected 36 → diff 6 ≤ 12 → match
	cv := Compare("x", 1, p, Expected{TotalExperienceMonths: 36})
	if cv.Fields[0].Score != 1 {
		t.Fatalf("within-tolerance experience should match, got %v", cv.Fields[0].Score)
	}
	// expected 60 → diff 30 > 12 → miss
	cv2 := Compare("x", 1, p, Expected{TotalExperienceMonths: 60})
	if cv2.Fields[0].Score != 0 {
		t.Fatalf("out-of-tolerance experience should miss, got %v", cv2.Fields[0].Score)
	}
}

func TestCompare_SkillsRecall(t *testing.T) {
	cv := Compare("x", 1, sampleProfile(), Expected{Skills: []string{"golang", "redis"}})
	// 1 of 2 expected skills present → recall 0.5
	if !approx(cv.Fields[0].Score, 0.5) {
		t.Fatalf("skills recall = %v, want 0.5", cv.Fields[0].Score)
	}
}

func TestCompare_EmptyExpectedSkipsField(t *testing.T) {
	cv := Compare("x", 1, sampleProfile(), Expected{}) // nothing to grade
	if len(cv.Fields) != 0 {
		t.Fatalf("empty expected should grade no fields, got %d", len(cv.Fields))
	}
}

func TestAggregate_PerFieldAndMacro(t *testing.T) {
	scores := []CVScore{
		{File: "1", OCRConfidence: 0.9, Fields: []FieldResult{{Field: "name", Score: 1}, {Field: "email", Score: 1}}},
		{File: "2", OCRConfidence: 0.6, Fields: []FieldResult{{Field: "name", Score: 0}, {Field: "email", Score: 1}}},
	}
	agg := AggregateScores(scores)
	if !approx(agg.PerField["name"], 0.5) || !approx(agg.PerField["email"], 1.0) {
		t.Fatalf("per-field wrong: %+v", agg.PerField)
	}
	if !approx(agg.MacroAverage, 0.75) {
		t.Fatalf("macro = %v, want 0.75", agg.MacroAverage)
	}
	if agg.OCRBelow070 != 1 {
		t.Fatalf("OCR below 0.70 count = %d, want 1", agg.OCRBelow070)
	}
}
