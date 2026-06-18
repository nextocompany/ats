package applications

import "testing"

func fbTA(rating int) InterviewFeedback {
	return InterviewFeedback{Perspective: PerspectiveTA, OverallRating: rating, Recommendation: RecPass,
		Competencies: InterviewCompetencies{Technical: 4, Communication: 3, Attitude: 5}}
}
func fbLM(rating int) InterviewFeedback {
	return InterviewFeedback{Perspective: PerspectiveLineManager, OverallRating: rating, Recommendation: RecHold,
		Competencies: InterviewCompetencies{CultureFit: 4, GrowthPotential: 5, Leadership: 3}}
}
func f64(v float64) *float64 { return &v }

func TestCompositeScore(t *testing.T) {
	cases := []struct {
		ai, ta float64
		want   float64
	}{
		{80, 0, 80}, // no TA → AI only
		{80, 4, 80}, // 80*.6 + 80*.4 = 80
		{90, 5, 94}, // 90*.6=54 + 100*.4=40 = 94
		{60, 3, 60}, // 60*.6=36 + 60*.4=24 = 60
		{0, 0, 0},
	}
	for _, c := range cases {
		if got := CompositeScore(c.ai, c.ta); got != c.want {
			t.Errorf("CompositeScore(%v,%v)=%v want %v", c.ai, c.ta, got, c.want)
		}
	}
}

func TestSummarizeFeedback_TAOnly(t *testing.T) {
	s := SummarizeFeedback([]InterviewFeedback{fbTA(4), fbTA(2)}, f64(80))
	if s.TA == nil || s.TA.Count != 2 {
		t.Fatalf("expected TA count 2, got %+v", s.TA)
	}
	if s.TA.AvgOverall != 3 {
		t.Fatalf("TA avg overall = %v want 3", s.TA.AvgOverall)
	}
	if s.LineManager != nil {
		t.Fatalf("expected no LM agg, got %+v", s.LineManager)
	}
	if s.CompositeScore == nil || *s.CompositeScore != CompositeScore(80, 3) {
		t.Fatalf("composite = %v want %v", s.CompositeScore, CompositeScore(80, 3))
	}
	// unrated dimensions must not appear
	if _, ok := s.TA.AvgCompetencies["culture_fit"]; ok {
		t.Fatalf("unrated culture_fit should be absent from TA averages")
	}
}

func TestSummarizeFeedback_Both(t *testing.T) {
	s := SummarizeFeedback([]InterviewFeedback{fbTA(4), fbLM(5)}, f64(90))
	if s.TA == nil || s.LineManager == nil {
		t.Fatalf("expected both aggregates, got TA=%v LM=%v", s.TA, s.LineManager)
	}
	if s.LineManager.AvgCompetencies["leadership"] != 3 {
		t.Fatalf("LM leadership avg = %v want 3", s.LineManager.AvgCompetencies["leadership"])
	}
	if *s.CompositeScore != CompositeScore(90, 4) {
		t.Fatalf("composite should use TA avg (4): got %v", *s.CompositeScore)
	}
}

func TestSummarizeFeedback_None(t *testing.T) {
	s := SummarizeFeedback(nil, f64(70))
	if s.TA != nil || s.LineManager != nil {
		t.Fatalf("expected nil aggs")
	}
	if s.CompositeScore == nil || *s.CompositeScore != 70 {
		t.Fatalf("composite with no TA should equal AI score 70, got %v", s.CompositeScore)
	}
}

func TestSummarizeFeedback_NoAIScore(t *testing.T) {
	s := SummarizeFeedback([]InterviewFeedback{fbTA(4)}, nil)
	if s.CompositeScore != nil {
		t.Fatalf("composite should be nil without an AI score, got %v", *s.CompositeScore)
	}
}

func TestSummarizeFeedback_LegacyRowCountsAsTA(t *testing.T) {
	legacy := InterviewFeedback{OverallRating: 5, Recommendation: RecPass} // no perspective
	s := SummarizeFeedback([]InterviewFeedback{legacy}, f64(50))
	if s.TA == nil || s.TA.Count != 1 {
		t.Fatalf("legacy (empty perspective) row should aggregate as TA, got %+v", s.TA)
	}
}
