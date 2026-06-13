package interview

import (
	"strings"
	"testing"
)

func TestParseEvaluation_ScoreAsNumber(t *testing.T) {
	in := `{"interview_score":81,"recommendation":"recommend","strengths":["ก","ข"],"concerns":["ค"],"summary":"ดี"}`
	ev, err := parseEvaluation(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.Score != 81 {
		t.Fatalf("want score 81, got %v", ev.Score)
	}
	if ev.Recommendation != "recommend" || len(ev.Strengths) != 2 || len(ev.Concerns) != 1 {
		t.Fatalf("unexpected eval: %+v", ev)
	}
}

func TestParseEvaluation_ScoreAsString(t *testing.T) {
	// gpt-4o-mini occasionally returns the score as a string — must not crash.
	in := `{"interview_score":"77","recommendation":"neutral","strengths":[],"concerns":[],"summary":"พอใช้"}`
	ev, err := parseEvaluation(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.Score != 77 {
		t.Fatalf("want score 77 from string, got %v", ev.Score)
	}
}

func TestParseEvaluation_ScoreClamped(t *testing.T) {
	ev, err := parseEvaluation(`{"interview_score":250,"recommendation":"strong_recommend"}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.Score != 100 {
		t.Fatalf("want clamped 100, got %v", ev.Score)
	}
}

func TestInterviewerSystemPrompt_GroundsInJD(t *testing.T) {
	p := interviewerSystemPrompt(InterviewContext{
		PositionTitle:    "พนักงานขาย",
		Responsibilities: "ดูแลหน้าร้าน",
		Qualifications:   "สื่อสารดี",
		MaxTurns:         5,
	})
	for _, want := range []string{"พนักงานขาย", "ดูแลหน้าร้าน", "สื่อสารดี", endSentinel} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}
