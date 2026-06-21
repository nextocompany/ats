package candidateauth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/pkg/email"
)

// --- in-memory fakes ---------------------------------------------------------

type fakeRepo struct {
	accounts map[uuid.UUID]*Account
	byEmail  map[string]uuid.UUID
	byLine   map[string]uuid.UUID
	byGoogle map[string]uuid.UUID
	sessions map[string]uuid.UUID // tokenHash -> accountID (live)
	otps     map[string]string    // email -> live codeHash (single, unconsumed)
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		accounts: map[uuid.UUID]*Account{},
		byEmail:  map[string]uuid.UUID{},
		byLine:   map[string]uuid.UUID{},
		byGoogle: map[string]uuid.UUID{},
		sessions: map[string]uuid.UUID{},
		otps:     map[string]string{},
	}
}

func (f *fakeRepo) newAccount() *Account {
	// Mirror the DB default (status NOT NULL DEFAULT 'active') so the suspension
	// guard in issueSessionFor doesn't reject freshly-created fake accounts.
	a := &Account{ID: uuid.New(), Status: statusActive, CreatedAt: time.Now()}
	f.accounts[a.ID] = a
	return a
}

func (f *fakeRepo) FindOrCreateByEmail(_ context.Context, email string) (*Account, error) {
	if id, ok := f.byEmail[email]; ok {
		f.accounts[id].EmailVerified = true
		return f.accounts[id], nil
	}
	a := f.newAccount()
	a.Email = email
	a.EmailVerified = true
	f.byEmail[email] = a.ID
	return a, nil
}

func (f *fakeRepo) findOrCreateSub(m map[string]uuid.UUID, sub, name, email string) (*Account, error) {
	if id, ok := m[sub]; ok {
		return f.accounts[id], nil
	}
	if email != "" {
		if id, ok := f.byEmail[email]; ok {
			m[sub] = id
			return f.accounts[id], nil
		}
	}
	a := f.newAccount()
	a.FullName = name
	a.Email = email
	m[sub] = a.ID
	if email != "" {
		f.byEmail[email] = a.ID
	}
	return a, nil
}

func (f *fakeRepo) FindOrCreateByLineSub(_ context.Context, sub, name, email string) (*Account, error) {
	a, err := f.findOrCreateSub(f.byLine, sub, name, email)
	if err == nil {
		a.LineUserID = sub
	}
	return a, err
}

func (f *fakeRepo) FindOrCreateByGoogleSub(_ context.Context, sub, name, email string) (*Account, error) {
	a, err := f.findOrCreateSub(f.byGoogle, sub, name, email)
	if err == nil {
		a.GoogleSub = sub
	}
	return a, err
}

func (f *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*Account, error) {
	if a, ok := f.accounts[id]; ok {
		return a, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) LinkLine(_ context.Context, accountID uuid.UUID, sub, displayID string) error {
	if _, ok := f.byLine[sub]; ok {
		return errors.New("line already linked")
	}
	a, ok := f.accounts[accountID]
	if !ok {
		return ErrNotFound
	}
	a.LineUserID = sub
	if displayID != "" {
		a.LineDisplayID = displayID
	}
	f.byLine[sub] = accountID
	return nil
}

func (f *fakeRepo) UpdateProfile(_ context.Context, id uuid.UUID, p ProfileUpdate) error {
	a, ok := f.accounts[id]
	if !ok {
		return ErrNotFound
	}
	if p.FullName != "" {
		a.FullName = p.FullName
	}
	if p.Phone != "" {
		a.Phone = p.Phone
	}
	if p.Province != "" {
		a.Province = p.Province
	}
	if p.LineDisplayID != "" {
		a.LineDisplayID = p.LineDisplayID
	}
	return nil
}

func (f *fakeRepo) SetResume(_ context.Context, id uuid.UUID, blobURL, fileType string) error {
	a, ok := f.accounts[id]
	if !ok {
		return ErrNotFound
	}
	a.ResumeBlobURL = blobURL
	a.ResumeFileType = fileType
	return nil
}

func (f *fakeRepo) SetConsent(_ context.Context, id uuid.UUID, version string) error {
	a, ok := f.accounts[id]
	if !ok {
		return ErrNotFound
	}
	a.PDPAConsent = true
	a.PDPAVersion = version
	return nil
}

func (f *fakeRepo) MarkConsented(_ context.Context, id uuid.UUID, version string) error {
	a, ok := f.accounts[id]
	if !ok {
		return ErrNotFound
	}
	a.PDPAConsent = true
	a.PDPAVersion = version
	return nil
}

func (f *fakeRepo) WithdrawConsent(_ context.Context, id uuid.UUID, _ string) error {
	a, ok := f.accounts[id]
	if !ok {
		return ErrNotFound
	}
	// Mirror writeConsent: only the boolean flips; the accepted version is kept.
	a.PDPAConsent = false
	return nil
}

func (f *fakeRepo) CreateSession(_ context.Context, accountID uuid.UUID, tokenHash string, _ time.Time) error {
	f.sessions[tokenHash] = accountID
	return nil
}

func (f *fakeRepo) FindAccountBySessionHash(_ context.Context, tokenHash string) (*Account, error) {
	if id, ok := f.sessions[tokenHash]; ok {
		return f.accounts[id], nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) RevokeSession(_ context.Context, tokenHash string) error {
	delete(f.sessions, tokenHash)
	return nil
}

func (f *fakeRepo) CreateOTP(_ context.Context, email, codeHash string, _ time.Time) error {
	f.otps[email] = codeHash
	return nil
}

func (f *fakeRepo) ConsumeOTP(_ context.Context, email, codeHash string) error {
	if h, ok := f.otps[email]; ok && h == codeHash {
		delete(f.otps, email)
		return nil
	}
	return ErrOTPInvalid
}

// recordingSender implements pkg/email.Sender by recording the plain-text body.
type recordingSender struct{ lastBody string }

func (r *recordingSender) Send(_ context.Context, m email.Message) error {
	r.lastBody = m.PlainText
	return nil
}

type fakeBlob struct{ uploaded map[string][]byte }

func newFakeBlob() *fakeBlob { return &fakeBlob{uploaded: map[string][]byte{}} }
func (b *fakeBlob) Upload(_ context.Context, name string, data []byte, _ string) (string, error) {
	b.uploaded[name] = data
	return "https://blob/" + name, nil
}
func (b *fakeBlob) Download(_ context.Context, name string) ([]byte, error) {
	if d, ok := b.uploaded[name]; ok {
		return d, nil
	}
	return nil, errors.New("not found")
}

// --- tests -------------------------------------------------------------------

func newTestService(repo Repository) (*Service, *recordingSender) {
	rs := &recordingSender{}
	svc := NewService(repo, rs, newFakeBlob(), 10*time.Minute, 720*time.Hour)
	return svc, rs
}

func TestStartEmailOTPStoresAndSends(t *testing.T) {
	repo := newFakeRepo()
	svc, rs := newTestService(repo)
	ctx := context.Background()

	if err := svc.StartEmailOTP(ctx, "User@Example.com"); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Address is normalized (lowercased) before storing the challenge.
	if _, ok := repo.otps["user@example.com"]; !ok {
		t.Fatal("expected a stored OTP challenge for the normalized email")
	}
	if rs.lastBody == "" {
		t.Fatal("expected an OTP email to be sent")
	}
}

func TestVerifyKnownCode(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()

	// Seed a known OTP directly through the repo (bypassing random generation).
	const code = "424242"
	if err := repo.CreateOTP(ctx, "a@b.com", hashSecret(code), time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("seed otp: %v", err)
	}
	acct, sess, err := svc.VerifyEmailOTP(ctx, "a@b.com", code)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if acct.Email != "a@b.com" || !acct.EmailVerified {
		t.Fatalf("unexpected account: %+v", acct)
	}
	if sess.Token == "" {
		t.Fatal("expected a session token")
	}
	// Session resolves back to the account.
	got, err := svc.AccountFromSession(ctx, sess.Token)
	if err != nil || got.ID != acct.ID {
		t.Fatalf("session did not resolve: %v %+v", err, got)
	}
	// Reuse of the same code now fails (consumed).
	if _, _, err := svc.VerifyEmailOTP(ctx, "a@b.com", code); !errors.Is(err, ErrOTPInvalid) {
		t.Fatalf("expected ErrOTPInvalid on reuse, got %v", err)
	}
}

func TestLoginWithLineCreatesSession(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()

	tok, exp, err := svc.LoginWithLine(ctx, "U123", "Somchai", "")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if tok == "" || exp.Before(time.Now()) {
		t.Fatalf("bad session: tok=%q exp=%v", tok, exp)
	}
	acct, err := svc.AccountFromSession(ctx, tok)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !acct.LineLinked() || acct.FullName != "Somchai" {
		t.Fatalf("unexpected account: %+v", acct)
	}
}

func TestLinkLineFromSession(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()

	// Email account first.
	_ = repo.CreateOTP(ctx, "e@x.com", hashSecret("111111"), time.Now().Add(time.Minute))
	_, sess, err := svc.VerifyEmailOTP(ctx, "e@x.com", "111111")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := svc.LinkLine(ctx, sess.Token, "U999", "@somchai"); err != nil {
		t.Fatalf("link: %v", err)
	}
	acct, _ := svc.AccountFromSession(ctx, sess.Token)
	if !acct.LineLinked() {
		t.Fatalf("expected line linked: %+v", acct)
	}
}

func TestLogoutRevokes(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()
	tok, _, _ := svc.LoginWithLine(ctx, "U1", "n", "")
	if err := svc.Logout(ctx, tok); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := svc.AccountFromSession(ctx, tok); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after logout, got %v", err)
	}
}

func TestSaveResumeAndBytes(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()
	a := repo.newAccount()
	if err := svc.SaveResume(ctx, a.ID, "cv.pdf", "pdf", "application/pdf", []byte("PDFDATA")); err != nil {
		t.Fatalf("save resume: %v", err)
	}
	got, err := repo.GetByID(ctx, a.ID)
	if err != nil || !got.HasResume() || got.ResumeFileType != "pdf" {
		t.Fatalf("resume not saved: %+v %v", got, err)
	}
	data, err := svc.SavedResumeBytes(ctx, got)
	if err != nil || string(data) != "PDFDATA" {
		t.Fatalf("download mismatch: %q %v", string(data), err)
	}
}

func TestStartEmailOTPInvalidAddress(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	if err := svc.StartEmailOTP(context.Background(), "not-an-email"); err == nil {
		t.Fatal("expected invalid email error")
	}
}

func TestOTPHelpers(t *testing.T) {
	code, err := genOTP()
	if err != nil {
		t.Fatalf("genOTP: %v", err)
	}
	if len(code) != otpDigits {
		t.Fatalf("expected %d digits, got %q", otpDigits, code)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			t.Fatalf("non-digit in otp: %q", code)
		}
	}
	h1, h2, h3 := hashSecret("abc"), hashSecret("abc"), hashSecret("abd")
	if h1 != h2 {
		t.Fatal("hashSecret not deterministic")
	}
	if h1 == h3 {
		t.Fatal("hashSecret collided on distinct inputs")
	}
	tok, err := randToken()
	if err != nil || strings.TrimSpace(tok) == "" {
		t.Fatalf("randToken: %q %v", tok, err)
	}
}
