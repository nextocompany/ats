package candidateauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
)

// otpDigits is the length of the email one-time code.
const otpDigits = 6

// genOTP returns a cryptographically-random zero-padded 6-digit code.
func genOTP() (string, error) {
	max := big.NewInt(1000000) // 000000–999999
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("candidateauth: otp gen: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// randToken returns a URL-safe 24-byte opaque token (mirrors the public/interview
// token convention).
func randToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("candidateauth: token gen: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashSecret returns the hex sha256 of a code/token — what we store at rest so a
// DB leak never exposes a live OTP or session token.
func hashSecret(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
