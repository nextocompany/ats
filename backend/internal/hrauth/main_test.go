package hrauth

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestMain lowers the bcrypt cost for the suite so the many hash/verify round
// trips run fast; production keeps the higher cost set in password.go.
func TestMain(m *testing.M) {
	bcryptCost = bcrypt.MinCost
	os.Exit(m.Run())
}
