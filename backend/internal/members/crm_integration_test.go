//go:build integration

package members

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/rbac"
)

func TestNotes_AddListAndMissingMember(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	id := m1ID(t, r)

	if _, err := r.AddNote(ctx, id, "hr@x.com", "first note"); err != nil {
		t.Fatalf("add note 1: %v", err)
	}
	if _, err := r.AddNote(ctx, id, "hr@x.com", "second note"); err != nil {
		t.Fatalf("add note 2: %v", err)
	}
	notes, err := r.ListNotes(ctx, id)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	if notes[0].Body != "second note" {
		t.Fatalf("notes should be newest-first, got %q first", notes[0].Body)
	}

	// A note on a non-existent member is rejected by the FK → ErrNotFound.
	if _, err := r.AddNote(ctx, uuid.New(), "hr@x.com", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("note on missing member want ErrNotFound, got %v", err)
	}
}

func TestTags_AddIdempotentRemoveList(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	id := m1ID(t, r)

	if err := r.AddTag(ctx, id, "retail"); err != nil {
		t.Fatalf("add tag: %v", err)
	}
	if err := r.AddTag(ctx, id, "retail"); err != nil { // idempotent
		t.Fatalf("re-add tag should be a no-op, got %v", err)
	}
	if err := r.AddTag(ctx, id, "north"); err != nil {
		t.Fatalf("add second tag: %v", err)
	}
	tags, err := r.ListTags(ctx, id)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(tags) != 2 || tags[0] != "north" || tags[1] != "retail" {
		t.Fatalf("expected [north retail] sorted, got %v", tags)
	}

	if err := r.RemoveTag(ctx, id, "retail"); err != nil {
		t.Fatalf("remove tag: %v", err)
	}
	if err := r.RemoveTag(ctx, id, "retail"); err != nil { // idempotent
		t.Fatalf("re-remove should be a no-op, got %v", err)
	}
	tags, _ = r.ListTags(ctx, id)
	if len(tags) != 1 || tags[0] != "north" {
		t.Fatalf("expected [north] after remove, got %v", tags)
	}

	if err := r.AddTag(ctx, uuid.New(), "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("tag on missing member want ErrNotFound, got %v", err)
	}
}

func TestList_TagFilter(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	id := m1ID(t, r)

	if err := r.AddTag(ctx, id, "vip"); err != nil {
		t.Fatalf("tag: %v", err)
	}
	if _, total, _ := r.List(ctx, ListFilter{Tag: "vip"}, rbac.AllScope()); total != 1 {
		t.Errorf("tag filter vip: want 1, got %d", total)
	}
	if _, total, _ := r.List(ctx, ListFilter{Tag: "nobody"}, rbac.AllScope()); total != 0 {
		t.Errorf("tag filter nobody: want 0, got %d", total)
	}
}

func TestListForExport_RespectsFilterAndCap(t *testing.T) {
	r := setup(t)
	ctx := context.Background()

	// setup seeds 4 members (1 suspended). Export honours the same filters as List.
	all, err := r.ListForExport(ctx, ListFilter{}, rbac.AllScope(), 1000)
	if err != nil {
		t.Fatalf("export all: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(all))
	}
	suspended, _ := r.ListForExport(ctx, ListFilter{Status: StatusSuspended}, rbac.AllScope(), 1000)
	if len(suspended) != 1 {
		t.Fatalf("export suspended: want 1, got %d", len(suspended))
	}
	// Cap bounds the result set.
	capped, _ := r.ListForExport(ctx, ListFilter{}, rbac.AllScope(), 2)
	if len(capped) != 2 {
		t.Fatalf("export cap=2: want 2, got %d", len(capped))
	}
}
