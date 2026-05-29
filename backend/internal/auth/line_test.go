package auth

import (
	"context"
	"testing"

	"github.com/nexto/hr-ats/pkg/config"
)

func TestMockVerifier(t *testing.T) {
	v := NewVerifier(&config.Config{LINEProvider: "mock"})

	if _, err := v.Verify(context.Background(), ""); err == nil {
		t.Error("expected error for empty token")
	}
	u, err := v.Verify(context.Background(), "dev-stub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Subject == "" {
		t.Error("expected a subject for a valid stub token")
	}
}

func TestNewVerifier_RealSelected(t *testing.T) {
	v := NewVerifier(&config.Config{LINEProvider: config.ProviderReal, LINEChannelID: "123"})
	if _, ok := v.(realVerifier); !ok {
		t.Error("expected realVerifier when LINE_PROVIDER=real")
	}
}
