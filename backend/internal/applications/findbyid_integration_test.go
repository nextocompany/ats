//go:build integration

package applications

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestFindByID_Explainability verifies the score-explainability columns added to
// the SELECT round-trip correctly: JSONB ai_score_breakdown unmarshals into the
// typed struct, text summary/red-flags come back, and the suggested-positions
// JSON array decodes. Reuses setupList's truncate + position/candidate seed.
func TestFindByID_Explainability(t *testing.T) {
	r, pos, cand := setupList(t)
	ctx := context.Background()

	var id uuid.UUID
	const ins = `
		INSERT INTO applications
			(candidate_id, position_id, status, ai_score, must_have_passed,
			 ai_score_breakdown, ai_summary, ai_red_flags, ai_suggested_positions)
		VALUES ($1,$2,'scored',81,true,
			'{"experience":20,"skills":16,"education":8,"language":8,"location":15}'::jsonb,
			'จุดแข็งหนึ่ง', 'ข้อสังเกตหนึ่ง',
			'["พนักงานแคชเชียร์","พนักงานบริการ"]'::jsonb)
		RETURNING id`
	if err := r.pool.QueryRow(ctx, ins, cand, pos).Scan(&id); err != nil {
		t.Fatalf("insert app: %v", err)
	}

	app, err := r.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if app.AIScoreBreakdown == nil {
		t.Fatal("expected breakdown, got nil")
	}
	want := ScoreBreakdown{Experience: 20, Skills: 16, Education: 8, Language: 8, Location: 15}
	if *app.AIScoreBreakdown != want {
		t.Errorf("breakdown = %+v, want %+v", *app.AIScoreBreakdown, want)
	}
	if app.AISummary != "จุดแข็งหนึ่ง" {
		t.Errorf("summary = %q", app.AISummary)
	}
	if app.AIRedFlags != "ข้อสังเกตหนึ่ง" {
		t.Errorf("red flags = %q", app.AIRedFlags)
	}
	if len(app.AISuggestedPositions) != 2 || app.AISuggestedPositions[0] != "พนักงานแคชเชียร์" {
		t.Errorf("suggested = %v", app.AISuggestedPositions)
	}
}

// An unscored application has NULL explainability columns; FindByID must leave
// the pointer/slice nil rather than erroring on the scan.
func TestFindByID_UnscoredHasNilExplainability(t *testing.T) {
	r, pos, cand := setupList(t)
	ctx := context.Background()

	var id uuid.UUID
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO applications (candidate_id, position_id, status) VALUES ($1,$2,'pending') RETURNING id`,
		cand, pos).Scan(&id); err != nil {
		t.Fatalf("insert app: %v", err)
	}

	app, err := r.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if app.AIScoreBreakdown != nil {
		t.Errorf("expected nil breakdown, got %+v", *app.AIScoreBreakdown)
	}
	if app.AISuggestedPositions != nil {
		t.Errorf("expected nil suggested, got %v", app.AISuggestedPositions)
	}
}
