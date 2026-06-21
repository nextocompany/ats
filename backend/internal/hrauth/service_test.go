package hrauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/auth"
)

// fakeRepo is an in-memory Repository for service tests.
type fakeRepo struct {
	users      map[string]userRow // keyed by lower(email)
	byID       map[uuid.UUID]string
	sessions   map[string]session // keyed by tokenHash
	touchCalls int
	ssoUpserts int
}

type userRow struct {
	u    User
	hash string
}

type session struct {
	userID  uuid.UUID
	expires time.Time
	revoked bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:    map[string]userRow{},
		byID:     map[uuid.UUID]string{},
		sessions: map[string]session{},
	}
}

func (f *fakeRepo) seed(email, password, role string, active bool) uuid.UUID {
	id := uuid.New()
	hash, _ := hashPassword(password)
	u := User{ID: id, Email: email, Role: role, IsActive: active, HasPassword: true}
	f.users[email] = userRow{u: u, hash: hash}
	f.byID[id] = email
	return id
}

func (f *fakeRepo) FindCredentialsByEmail(_ context.Context, email string) (User, string, error) {
	r, ok := f.users[email]
	if !ok || !r.u.IsActive {
		return User{}, "", ErrNotFound
	}
	return r.u, r.hash, nil
}

func (f *fakeRepo) TouchLastLogin(_ context.Context, _ uuid.UUID) error { f.touchCalls++; return nil }

func (f *fakeRepo) CreateSession(_ context.Context, userID uuid.UUID, tokenHash string, exp time.Time) error {
	f.sessions[tokenHash] = session{userID: userID, expires: exp}
	return nil
}

func (f *fakeRepo) FindSessionUser(_ context.Context, tokenHash string) (User, error) {
	s, ok := f.sessions[tokenHash]
	if !ok || s.revoked || s.expires.Before(time.Now()) {
		return User{}, ErrNotFound
	}
	email := f.byID[s.userID]
	r := f.users[email]
	if !r.u.IsActive {
		return User{}, ErrNotFound
	}
	return r.u, nil
}

func (f *fakeRepo) RevokeSession(_ context.Context, tokenHash string) error {
	if s, ok := f.sessions[tokenHash]; ok {
		s.revoked = true
		f.sessions[tokenHash] = s
	}
	return nil
}

func (f *fakeRepo) RevokeAllUserSessions(_ context.Context, userID uuid.UUID) error {
	for h, s := range f.sessions {
		if s.userID == userID {
			s.revoked = true
			f.sessions[h] = s
		}
	}
	return nil
}

func (f *fakeRepo) UpsertSSOUser(_ context.Context, _, _, _ string) error { f.ssoUpserts++; return nil }

func (f *fakeRepo) ListUsers(context.Context) ([]User, error) {
	out := []User{}
	for _, r := range f.users {
		out = append(out, r.u)
	}
	return out, nil
}

func (f *fakeRepo) GetUser(_ context.Context, id uuid.UUID) (User, error) {
	email, ok := f.byID[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return f.users[email].u, nil
}

func (f *fakeRepo) CreateUser(_ context.Context, email, fullName, role string, storeID *int, subregion, hash string) (User, error) {
	if _, exists := f.users[email]; exists {
		return User{}, ErrEmailExists
	}
	id := uuid.New()
	u := User{ID: id, Email: email, FullName: fullName, Role: role, StoreID: storeID, Subregion: subregion, IsActive: true, HasPassword: true}
	f.users[email] = userRow{u: u, hash: hash}
	f.byID[id] = email
	return u, nil
}

func (f *fakeRepo) UpdateUser(_ context.Context, id uuid.UUID, in UpdateUserInput, hash *string) (User, error) {
	email, ok := f.byID[id]
	if !ok {
		return User{}, ErrNotFound
	}
	r := f.users[email]
	if in.Role != nil {
		r.u.Role = *in.Role
	}
	if in.IsActive != nil {
		r.u.IsActive = *in.IsActive
	}
	if hash != nil {
		r.hash = *hash
	}
	f.users[email] = r
	return r.u, nil
}

func newSvc(repo Repository) *Service { return NewService(repo, time.Hour) }

func TestProvisionSSOUser_SkipsEmptyOID(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	// A local password session carries a user UUID in Identity.ID via ValidateSession,
	// but the SSO path is the only caller; an empty oid must be a no-op.
	if err := svc.ProvisionSSOUser(context.Background(), auth.Identity{ID: ""}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.ssoUpserts != 0 {
		t.Fatalf("expected no upsert for empty oid, got %d", repo.ssoUpserts)
	}
}

func TestProvisionSSOUser_Throttles(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo)
	clock := time.Unix(1_700_000_000, 0)
	svc.now = func() time.Time { return clock }
	id := auth.Identity{ID: "oid-1", Email: "Dpo@CPAxtra.com", Name: "DPO One"}

	// First call provisions.
	_ = svc.ProvisionSSOUser(context.Background(), id)
	// Second call within the throttle window is skipped (no DB write).
	_ = svc.ProvisionSSOUser(context.Background(), id)
	if repo.ssoUpserts != 1 {
		t.Fatalf("expected 1 upsert within throttle window, got %d", repo.ssoUpserts)
	}

	// After the window elapses, it provisions again.
	clock = clock.Add(ssoProvisionThrottle + time.Minute)
	_ = svc.ProvisionSSOUser(context.Background(), id)
	if repo.ssoUpserts != 2 {
		t.Fatalf("expected 2 upserts after throttle window, got %d", repo.ssoUpserts)
	}
}

func TestLoginSuccess(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("hr@cpaxtra.com", "screening99", "hr_manager", true)
	svc := newSvc(repo)

	tok, exp, user, err := svc.Login(context.Background(), "HR@cpaxtra.com", "screening99")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tok == "" {
		t.Fatal("expected a session token")
	}
	if !exp.After(time.Now()) {
		t.Fatal("expected a future expiry")
	}
	if user.Email != "hr@cpaxtra.com" || user.Role != "hr_manager" {
		t.Fatalf("unexpected user: %+v", user)
	}
	if repo.touchCalls != 1 {
		t.Fatalf("expected last-login touch, got %d", repo.touchCalls)
	}
}

func TestLoginWrongPasswordIsGeneric(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("hr@cpaxtra.com", "screening99", "hr_manager", true)
	svc := newSvc(repo)

	_, _, _, err := svc.Login(context.Background(), "hr@cpaxtra.com", "wrongpassword1")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUnknownEmailIsGeneric(t *testing.T) {
	svc := newSvc(newFakeRepo())
	_, _, _, err := svc.Login(context.Background(), "ghost@cpaxtra.com", "whatever123")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials for unknown email, got %v", err)
	}
}

func TestLoginInactiveUserRejected(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("ex@cpaxtra.com", "screening99", "hr_manager", false)
	svc := newSvc(repo)
	_, _, _, err := svc.Login(context.Background(), "ex@cpaxtra.com", "screening99")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials for inactive user, got %v", err)
	}
}

func TestValidateSessionRoundTrip(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("hr@cpaxtra.com", "screening99", "super_admin", true)
	svc := newSvc(repo)

	tok, _, _, err := svc.Login(context.Background(), "hr@cpaxtra.com", "screening99")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	id, ok := svc.ValidateSession(context.Background(), tok)
	if !ok {
		t.Fatal("expected the freshly-minted session to validate")
	}
	if id.Role != "super_admin" || id.Email != "hr@cpaxtra.com" {
		t.Fatalf("unexpected identity: %+v", id)
	}
	if _, ok := svc.ValidateSession(context.Background(), "garbage-token"); ok {
		t.Fatal("a bogus token must not validate")
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("hr@cpaxtra.com", "screening99", "hr_staff", true)
	svc := newSvc(repo)

	tok, _, _, _ := svc.Login(context.Background(), "hr@cpaxtra.com", "screening99")
	if err := svc.Logout(context.Background(), tok); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, ok := svc.ValidateSession(context.Background(), tok); ok {
		t.Fatal("session must not validate after logout")
	}
}

func TestCreateUserValidation(t *testing.T) {
	svc := newSvc(newFakeRepo())

	if _, err := svc.CreateUser(context.Background(), NewUserInput{Email: "a@b.com", Role: "wizard", Password: "screening99"}); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("want ErrInvalidRole, got %v", err)
	}
	if _, err := svc.CreateUser(context.Background(), NewUserInput{Email: "a@b.com", Role: "hr_staff", Password: "weak"}); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("want ErrWeakPassword, got %v", err)
	}
	u, err := svc.CreateUser(context.Background(), NewUserInput{Email: "New@B.com", Role: "hr_staff", Password: "screening99"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Email != "new@b.com" {
		t.Fatalf("email should be normalised, got %q", u.Email)
	}
}

func TestUpdateUserPasswordResetRevokesSessions(t *testing.T) {
	repo := newFakeRepo()
	id := repo.seed("hr@cpaxtra.com", "screening99", "hr_manager", true)
	svc := newSvc(repo)

	tok, _, _, _ := svc.Login(context.Background(), "hr@cpaxtra.com", "screening99")
	newPw := "rotated-pw-2026"
	if _, err := svc.UpdateUser(context.Background(), id, otherAdmin, UpdateUserInput{Password: &newPw}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if _, ok := svc.ValidateSession(context.Background(), tok); ok {
		t.Fatal("a password reset must revoke live sessions")
	}
}

// otherAdmin is a caller id distinct from any seeded user, so the self-lockout
// guard does not trip in tests that edit a different account.
const otherAdmin = "11111111-1111-1111-1111-111111111111"

func TestUpdateUserDeactivateRevokesSessions(t *testing.T) {
	repo := newFakeRepo()
	id := repo.seed("hr@cpaxtra.com", "screening99", "hr_manager", true)
	svc := newSvc(repo)

	tok, _, _, _ := svc.Login(context.Background(), "hr@cpaxtra.com", "screening99")
	inactive := false
	if _, err := svc.UpdateUser(context.Background(), id, otherAdmin, UpdateUserInput{IsActive: &inactive}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if _, ok := svc.ValidateSession(context.Background(), tok); ok {
		t.Fatal("deactivating a user must revoke live sessions")
	}
}

func TestUpdateUserSelfLockoutRejected(t *testing.T) {
	repo := newFakeRepo()
	id := repo.seed("boss@cpaxtra.com", "screening99", "super_admin", true)
	svc := newSvc(repo)

	inactive := false
	if _, err := svc.UpdateUser(context.Background(), id, id.String(), UpdateUserInput{IsActive: &inactive}); !errors.Is(err, ErrSelfLockout) {
		t.Fatalf("disabling own account: want ErrSelfLockout, got %v", err)
	}
	demote := "hr_staff"
	if _, err := svc.UpdateUser(context.Background(), id, id.String(), UpdateUserInput{Role: &demote}); !errors.Is(err, ErrSelfLockout) {
		t.Fatalf("demoting own account: want ErrSelfLockout, got %v", err)
	}
	// Editing one's own name (no demote/disable) is allowed.
	name := "The Boss"
	if _, err := svc.UpdateUser(context.Background(), id, id.String(), UpdateUserInput{FullName: &name}); err != nil {
		t.Fatalf("self name edit should be allowed, got %v", err)
	}
}
