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
	resumes  map[uuid.UUID][]Resume
	keys     map[uuid.UUID]string // resumeID -> blob key
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		accounts: map[uuid.UUID]*Account{},
		byEmail:  map[string]uuid.UUID{},
		byLine:   map[string]uuid.UUID{},
		byGoogle: map[string]uuid.UUID{},
		sessions: map[string]uuid.UUID{},
		otps:     map[string]string{},
		resumes:  map[uuid.UUID][]Resume{},
		keys:     map[uuid.UUID]string{},
	}
}

func (f *fakeRepo) ListResumes(_ context.Context, accountID uuid.UUID) ([]Resume, error) {
	return f.resumes[accountID], nil
}

func (f *fakeRepo) CountResumes(_ context.Context, accountID uuid.UUID) (int, error) {
	return len(f.resumes[accountID]), nil
}

func (f *fakeRepo) InsertResume(_ context.Context, accountID, id uuid.UUID, blobKey, filename, fileType string, makeDefault bool) error {
	// Prepend so the slice stays newest-first, matching the SQL ORDER BY.
	f.resumes[accountID] = append([]Resume{{ID: id, OriginalFilename: filename, FileType: fileType, IsDefault: makeDefault, CreatedAt: time.Now()}}, f.resumes[accountID]...)
	f.keys[id] = blobKey
	if makeDefault {
		f.accounts[accountID].ResumeBlobURL = blobKey
		f.accounts[accountID].ResumeFileType = fileType
	}
	return nil
}

func (f *fakeRepo) SetDefaultResume(_ context.Context, accountID, resumeID uuid.UUID) error {
	list := f.resumes[accountID]
	found := false
	for i := range list {
		list[i].IsDefault = list[i].ID == resumeID
		if list[i].ID == resumeID {
			found = true
			f.accounts[accountID].ResumeBlobURL = f.keys[resumeID]
			f.accounts[accountID].ResumeFileType = list[i].FileType
		}
	}
	if !found {
		return ErrNotFound
	}
	return nil
}

func (f *fakeRepo) DeleteResume(_ context.Context, accountID, resumeID uuid.UUID) (string, error) {
	list := f.resumes[accountID]
	idx := -1
	for i := range list {
		if list[i].ID == resumeID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", ErrNotFound
	}
	wasDefault := list[idx].IsDefault
	key := f.keys[resumeID]
	f.resumes[accountID] = append(list[:idx], list[idx+1:]...)
	delete(f.keys, resumeID)
	if wasDefault && len(f.resumes[accountID]) > 0 {
		nl := f.resumes[accountID]
		nl[0].IsDefault = true // newest-first → index 0
		f.accounts[accountID].ResumeBlobURL = f.keys[nl[0].ID]
		f.accounts[accountID].ResumeFileType = nl[0].FileType
	} else if wasDefault {
		f.accounts[accountID].ResumeBlobURL = ""
		f.accounts[accountID].ResumeFileType = ""
	}
	return key, nil
}

func (f *fakeRepo) ResumeBlobKey(_ context.Context, accountID, resumeID uuid.UUID) (string, string, error) {
	for _, r := range f.resumes[accountID] {
		if r.ID == resumeID {
			return f.keys[resumeID], r.FileType, nil
		}
	}
	return "", "", ErrNotFound
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

func (f *fakeRepo) FindOrCreateUnverifiedByEmail(_ context.Context, email string) (*Account, error) {
	if id, ok := f.byEmail[email]; ok {
		return f.accounts[id], nil // never flip email_verified
	}
	a := f.newAccount()
	a.Email = email
	a.EmailVerified = false
	f.byEmail[email] = a.ID
	return a, nil
}

func (f *fakeRepo) findOrCreateSub(m map[string]uuid.UUID, sub, name, email string) (*Account, error) {
	if id, ok := m[sub]; ok {
		a := f.accounts[id]
		if a.Email == "" && email != "" { // backfill a newly-available email (set-once)
			a.Email = email
			f.byEmail[email] = a.ID
		}
		return a, nil
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
	if p.Email != "" && a.Email == "" {
		a.Email = p.Email
	}
	return nil
}

func (f *fakeRepo) BackfillContact(_ context.Context, id uuid.UUID, phone, email string) error {
	a, ok := f.accounts[id]
	if !ok {
		return ErrNotFound
	}
	if phone != "" && a.Phone == "" {
		a.Phone = phone
	}
	if email != "" && a.Email == "" {
		a.Email = email
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

// recordingSender implements pkg/email.Sender by recording the last message.
type recordingSender struct {
	lastBody string
	lastHTML string
}

func (r *recordingSender) Send(_ context.Context, m email.Message) error {
	r.lastBody = m.PlainText
	r.lastHTML = m.HTML
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
func (b *fakeBlob) Delete(_ context.Context, name string) error {
	delete(b.uploaded, name)
	return nil
}
func (b *fakeBlob) SignedURL(name string, _ time.Duration) (string, error) {
	return "https://blob/" + name + "?sig=test", nil
}

// --- tests -------------------------------------------------------------------

func newTestService(repo Repository) (*Service, *recordingSender) {
	rs := &recordingSender{}
	svc := NewService(repo, rs, newFakeBlob(), 10*time.Minute, 720*time.Hour)
	return svc, rs
}

func TestEnsureAccountByEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("creates an unverified account for a new email", func(t *testing.T) {
		repo := newFakeRepo()
		svc, _ := newTestService(repo)
		id, ok, err := svc.EnsureAccountByEmail(ctx, "New@Example.com")
		if err != nil || !ok {
			t.Fatalf("ensure: ok=%v err=%v", ok, err)
		}
		// Normalized (lowercased) key, account created unverified.
		acctID, found := repo.byEmail["new@example.com"]
		if !found || acctID != id {
			t.Fatalf("expected normalized account for new@example.com, got id=%v found=%v", acctID, found)
		}
		if repo.accounts[id].EmailVerified {
			t.Fatal("a CV-provisioned account must be unverified")
		}
	})

	t.Run("is idempotent and never flips an existing verified account", func(t *testing.T) {
		repo := newFakeRepo()
		svc, _ := newTestService(repo)
		// Pre-existing VERIFIED account (e.g. the member logged in via OTP earlier).
		verified, _ := repo.FindOrCreateByEmail(ctx, "member@example.com")
		id, ok, err := svc.EnsureAccountByEmail(ctx, "member@example.com")
		if err != nil || !ok {
			t.Fatalf("ensure: ok=%v err=%v", ok, err)
		}
		if id != verified.ID {
			t.Fatalf("expected the existing account %v, got %v", verified.ID, id)
		}
		if !repo.accounts[id].EmailVerified {
			t.Fatal("ensure must NOT flip an already-verified account to unverified")
		}
	})

	t.Run("skips empty or invalid email without error", func(t *testing.T) {
		repo := newFakeRepo()
		svc, _ := newTestService(repo)
		for _, raw := range []string{"", "   ", "not-an-email"} {
			id, ok, err := svc.EnsureAccountByEmail(ctx, raw)
			if err != nil {
				t.Fatalf("ensure(%q): unexpected error %v", raw, err)
			}
			if ok || id != uuid.Nil {
				t.Fatalf("ensure(%q): expected (Nil,false), got (%v,%v)", raw, id, ok)
			}
		}
		if len(repo.byEmail) != 0 {
			t.Fatal("no account should be created for an invalid email")
		}
	})
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
	// The OTP email is branded and bilingual (TH + EN).
	if rs.lastHTML == "" {
		t.Fatal("expected a branded HTML body for the OTP email")
	}
	for _, want := range []string{"CP", "AXTRA", "รหัส", "Code"} {
		if !strings.Contains(rs.lastHTML, want) {
			t.Errorf("OTP HTML missing %q", want)
		}
	}
	// The code must appear in both parts (sanity: bilingual plain + branded HTML
	// carry the same code), and never in a way that the mock would log as HTML.
	if !strings.Contains(rs.lastBody, "Code") {
		t.Errorf("OTP plain body should be bilingual: %q", rs.lastBody)
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

func TestResumeLibrary_DefaultCapPromote(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()
	acct := repo.newAccount()

	// First upload becomes the default.
	if err := svc.AddResume(ctx, acct.ID, "a.pdf", "pdf", "application/pdf", []byte("a")); err != nil {
		t.Fatal(err)
	}
	list, _ := svc.ListResumes(ctx, acct.ID)
	if len(list) != 1 || !list[0].IsDefault {
		t.Fatalf("first resume must be default, got %+v", list)
	}

	// Fill to MaxResumes; the 6th is blocked.
	for _, n := range []string{"2.pdf", "3.pdf", "4.pdf", "5.pdf"} {
		if err := svc.AddResume(ctx, acct.ID, n, "pdf", "application/pdf", []byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.AddResume(ctx, acct.ID, "x.pdf", "pdf", "application/pdf", []byte("x")); !errors.Is(err, ErrResumeLimit) {
		t.Fatalf("expected ErrResumeLimit at %d, got %v", MaxResumes, err)
	}
	list, _ = svc.ListResumes(ctx, acct.ID)
	if len(list) != MaxResumes || countDefaults(list) != 1 {
		t.Fatalf("want %d resumes with exactly 1 default, got %d/%d", MaxResumes, len(list), countDefaults(list))
	}

	// Switch the default to the newest (index 0), pointer must follow.
	newest := list[0]
	if err := svc.SetDefaultResume(ctx, acct.ID, newest.ID); err != nil {
		t.Fatal(err)
	}
	if repo.accounts[acct.ID].ResumeBlobURL == "" {
		t.Fatal("default pointer not synced after set-default")
	}
	list, _ = svc.ListResumes(ctx, acct.ID)
	for _, r := range list {
		if (r.ID == newest.ID) != r.IsDefault {
			t.Fatalf("default not switched correctly: %+v", r)
		}
	}

	// Deleting the default promotes another; exactly one default remains.
	if err := svc.DeleteResume(ctx, acct.ID, newest.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = svc.ListResumes(ctx, acct.ID)
	if len(list) != MaxResumes-1 || countDefaults(list) != 1 {
		t.Fatalf("after deleting default want %d with 1 default, got %d/%d", MaxResumes-1, len(list), countDefaults(list))
	}

	// Unknown id is a not-found, not a silent success.
	if err := svc.SetDefaultResume(ctx, acct.ID, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("set-default unknown id: want ErrNotFound, got %v", err)
	}
}

func TestResumeViewURL_OwnerAndIsolation(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo)
	ctx := context.Background()
	owner := repo.newAccount()
	other := repo.newAccount()

	if err := svc.AddResume(ctx, owner.ID, "cv.pdf", "pdf", "application/pdf", []byte("a")); err != nil {
		t.Fatal(err)
	}
	list, _ := svc.ListResumes(ctx, owner.ID)
	rid := list[0].ID

	// Owner gets a signed URL.
	url, err := svc.ResumeViewURL(ctx, owner.ID, rid)
	if err != nil {
		t.Fatalf("owner view: %v", err)
	}
	if !strings.Contains(url, "sig=") {
		t.Fatalf("want signed url, got %q", url)
	}

	// Another account cannot resolve the owner's resume (no IDOR).
	if _, err := svc.ResumeViewURL(ctx, other.ID, rid); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-account view: want ErrNotFound, got %v", err)
	}

	// Unknown id is a not-found, not a server error.
	if _, err := svc.ResumeViewURL(ctx, owner.ID, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id: want ErrNotFound, got %v", err)
	}
}

func countDefaults(list []Resume) int {
	n := 0
	for _, r := range list {
		if r.IsDefault {
			n++
		}
	}
	return n
}
