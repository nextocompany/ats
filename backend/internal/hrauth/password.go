package hrauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sync"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const (
	// minPasswordLen / maxPasswordLen bound the policy. bcrypt itself only
	// considers the first 72 bytes, so anything longer is rejected rather than
	// silently truncated.
	minPasswordLen = 10
	maxPasswordLen = 72
	// sessionTokenBytes is the entropy of an opaque session token.
	sessionTokenBytes = 32
)

// bcryptCost is deliberately above the library default (10) — the admin console
// is a high-value target and logins are infrequent. It is a var, not a const, so
// tests can lower it (bcrypt at cost 12 is ~250ms per hash, which would make the
// suite crawl); production never changes it.
var bcryptCost = 12

// hashPassword returns the bcrypt hash to store at rest.
func hashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hrauth: hash password: %w", err)
	}
	return string(b), nil
}

// checkPassword reports whether pw matches the stored bcrypt hash. A constant-time
// comparison is built into bcrypt.CompareHashAndPassword.
func checkPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// validatePassword enforces the password policy: a sensible length plus at least
// one letter and one digit. Kept simple and explainable rather than a rule soup
// that pushes users toward predictable patterns.
func validatePassword(pw string) error {
	if len(pw) < minPasswordLen || len(pw) > maxPasswordLen {
		return ErrWeakPassword
	}
	var hasLetter, hasDigit bool
	for _, r := range pw {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrWeakPassword
	}
	return nil
}

// genToken returns a URL-safe opaque session token (the plaintext goes in the
// cookie; only its hash is persisted).
func genToken() (string, error) {
	b := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("hrauth: token gen: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken returns the hex sha256 of a session token — what we store so a DB
// leak never exposes a live session.
func hashToken(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// dummyHash returns a valid bcrypt hash compared against when no user/password
// matches, so a missing account takes about the same time as a wrong password —
// closing the timing side-channel that would otherwise enumerate valid emails. It
// is the hash of an unknowable string; nothing can match it. Computed lazily via
// sync.Once so it honours the test-lowered bcryptCost (a package-init var would
// bake in the production cost before TestMain runs).
var (
	dummyHashOnce  sync.Once
	dummyHashValue string
)

func dummyHash() string {
	dummyHashOnce.Do(func() {
		h, err := hashPassword("hrauth-nonexistent-account-timing-equalizer")
		if err != nil {
			panic(err)
		}
		dummyHashValue = h
	})
	return dummyHashValue
}
