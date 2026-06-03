package ratelimit

import (
	"testing"

	"github.com/gofiber/fiber/v2"
)

// Compile-time proof that RedisStore satisfies the fiber.Storage contract.
var _ fiber.Storage = (*RedisStore)(nil)

func TestRedisStore_EmptyKeyIsNoop(t *testing.T) {
	// A nil client is safe here: every branch returns before dereferencing it.
	s := New(nil)

	got, err := s.Get("")
	if err != nil || got != nil {
		t.Errorf("Get(\"\") = (%v, %v), want (nil, nil)", got, err)
	}
	if err := s.Set("", []byte("x"), 0); err != nil {
		t.Errorf("Set(\"\", ...) = %v, want nil", err)
	}
	if err := s.Set("k", nil, 0); err != nil {
		t.Errorf("Set(k, nil, ...) = %v, want nil", err)
	}
	if err := s.Delete(""); err != nil {
		t.Errorf("Delete(\"\") = %v, want nil", err)
	}
}

func TestRedisStore_CloseIsNoop(t *testing.T) {
	if err := New(nil).Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}
