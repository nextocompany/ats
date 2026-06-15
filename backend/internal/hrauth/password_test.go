package hrauth

import "testing"

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name string
		pw   string
		ok   bool
	}{
		{"too short", "Ab1", false},
		{"letters only", "abcdefghij", false},
		{"digits only", "1234567890", false},
		{"valid", "screening99", true},
		{"valid mixed", "Lotus-Makro-2026", true},
		{"too long", string(make([]byte, 73)), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePassword(tc.pw)
			if tc.ok && err != nil {
				t.Fatalf("validatePassword(%q) = %v, want nil", tc.pw, err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("validatePassword(%q) = nil, want error", tc.pw)
			}
		})
	}
}

func TestHashAndCheckPassword(t *testing.T) {
	const pw = "screening99"
	hash, err := hashPassword(pw)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if hash == pw {
		t.Fatal("hash must not equal plaintext")
	}
	if !checkPassword(hash, pw) {
		t.Fatal("checkPassword should accept the correct password")
	}
	if checkPassword(hash, "wrongpassword1") {
		t.Fatal("checkPassword should reject a wrong password")
	}
}

func TestHashTokenDeterministicAndOpaque(t *testing.T) {
	tok, err := genToken()
	if err != nil {
		t.Fatalf("genToken: %v", err)
	}
	tok2, _ := genToken()
	if tok == tok2 {
		t.Fatal("genToken should produce unique tokens")
	}
	h := hashToken(tok)
	if h == tok {
		t.Fatal("hashToken must not return the plaintext token")
	}
	if h != hashToken(tok) {
		t.Fatal("hashToken must be deterministic")
	}
}
