//go:build integration

package candidateauth

import (
	"context"
	"errors"
	"testing"
)

// TestBackfillContact_FillsEmptyContact proves a LINE account with neither phone
// nor email gets both from an apply, and email_verified stays FALSE.
func TestBackfillContact_FillsEmptyContact(t *testing.T) {
	ctx := context.Background()
	pool := freshAuthDB(t)
	repo := NewRepository(pool)

	acct, err := repo.FindOrCreateByLineSub(ctx, "U-line-1", "LINE User", "")
	if err != nil {
		t.Fatalf("create line account: %v", err)
	}
	if acct.Email != "" || acct.Phone != "" {
		t.Fatalf("expected empty contact, got email=%q phone=%q", acct.Email, acct.Phone)
	}

	if err := repo.BackfillContact(ctx, acct.ID, "0812345678", "user@example.com"); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	got, err := repo.GetByID(ctx, acct.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Phone != "0812345678" {
		t.Errorf("phone = %q, want 0812345678", got.Phone)
	}
	if got.Email != "user@example.com" {
		t.Errorf("email = %q, want user@example.com", got.Email)
	}
	if got.EmailVerified {
		t.Error("email_verified flipped to TRUE — an apply email is not verified")
	}
}

// TestBackfillContact_DoesNotOverwrite proves backfill is fill-once: an account
// that already has phone+email keeps them.
func TestBackfillContact_DoesNotOverwrite(t *testing.T) {
	ctx := context.Background()
	pool := freshAuthDB(t)
	repo := NewRepository(pool)

	acct, err := repo.FindOrCreateByEmail(ctx, "owner@example.com") // verified, has email
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.BackfillContact(ctx, acct.ID, "0800000000", ""); err != nil {
		t.Fatalf("seed phone: %v", err)
	}

	if err := repo.BackfillContact(ctx, acct.ID, "0899999999", "new@example.com"); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	got, _ := repo.GetByID(ctx, acct.ID)
	if got.Phone != "0800000000" {
		t.Errorf("phone overwritten: %q, want 0800000000", got.Phone)
	}
	if got.Email != "owner@example.com" {
		t.Errorf("email overwritten: %q, want owner@example.com", got.Email)
	}
}

// TestBackfillContact_SkipsTakenEmail proves a collision (email owned by another
// account) is skipped without error, and the phone still fills.
func TestBackfillContact_SkipsTakenEmail(t *testing.T) {
	ctx := context.Background()
	pool := freshAuthDB(t)
	repo := NewRepository(pool)

	if _, err := repo.FindOrCreateByEmail(ctx, "taken@example.com"); err != nil {
		t.Fatalf("seed other: %v", err)
	}
	line, err := repo.FindOrCreateByLineSub(ctx, "U-line-2", "LINE Two", "")
	if err != nil {
		t.Fatalf("create line: %v", err)
	}

	if err := repo.BackfillContact(ctx, line.ID, "0855555555", "taken@example.com"); err != nil {
		t.Fatalf("backfill must not error on a taken email: %v", err)
	}

	got, _ := repo.GetByID(ctx, line.ID)
	if got.Email != "" {
		t.Errorf("email = %q, want empty (it belongs to another account)", got.Email)
	}
	if got.Phone != "0855555555" {
		t.Errorf("phone = %q, want 0855555555 (phone still fills)", got.Phone)
	}
}

// TestUpdateProfile_SetsEmailOnce proves the editor sets an email on an empty
// account, and a later edit with a different email is ignored (set-once).
func TestUpdateProfile_SetsEmailOnce(t *testing.T) {
	ctx := context.Background()
	pool := freshAuthDB(t)
	repo := NewRepository(pool)

	line, err := repo.FindOrCreateByLineSub(ctx, "U-line-3", "LINE Three", "")
	if err != nil {
		t.Fatalf("create line: %v", err)
	}

	if err := repo.UpdateProfile(ctx, line.ID, ProfileUpdate{Email: "first@example.com"}); err != nil {
		t.Fatalf("set email: %v", err)
	}
	if err := repo.UpdateProfile(ctx, line.ID, ProfileUpdate{Email: "second@example.com"}); err != nil {
		t.Fatalf("second edit: %v", err)
	}

	got, _ := repo.GetByID(ctx, line.ID)
	if got.Email != "first@example.com" {
		t.Errorf("email = %q, want first@example.com (set-once)", got.Email)
	}
	if got.EmailVerified {
		t.Error("email_verified flipped — a typed email is not verified")
	}
}

// TestUpdateProfile_RejectsTakenEmail proves the editor returns ErrEmailTaken when
// the email belongs to a different account (the 409 path).
func TestUpdateProfile_RejectsTakenEmail(t *testing.T) {
	ctx := context.Background()
	pool := freshAuthDB(t)
	repo := NewRepository(pool)

	if _, err := repo.FindOrCreateByEmail(ctx, "mine@example.com"); err != nil {
		t.Fatalf("seed other: %v", err)
	}
	line, err := repo.FindOrCreateByLineSub(ctx, "U-line-4", "LINE Four", "")
	if err != nil {
		t.Fatalf("create line: %v", err)
	}

	err = repo.UpdateProfile(ctx, line.ID, ProfileUpdate{Email: "mine@example.com"})
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken (the 409 path)", err)
	}
	got, _ := repo.GetByID(ctx, line.ID)
	if got.Email != "" {
		t.Errorf("email = %q, want empty (collision must not write)", got.Email)
	}
}
