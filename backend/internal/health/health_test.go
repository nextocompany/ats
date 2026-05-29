package health

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestEvaluate_AllOK(t *testing.T) {
	// Arrange
	checkers := []Checker{
		NewChecker("postgres", func(context.Context) error { return nil }),
		NewChecker("redis", func(context.Context) error { return nil }),
		NewChecker("blob", func(context.Context) error { return nil }),
	}

	// Act
	res := Evaluate(context.Background(), checkers)

	// Assert
	if !res.Healthy {
		t.Fatalf("expected healthy, got unhealthy: %v", res.Checks)
	}
	for _, name := range []string{"postgres", "redis", "blob"} {
		if res.Checks[name] != "ok" {
			t.Errorf("expected %s ok, got %q", name, res.Checks[name])
		}
	}
}

func TestEvaluate_OneFailing(t *testing.T) {
	// Arrange
	checkers := []Checker{
		NewChecker("postgres", func(context.Context) error { return nil }),
		NewChecker("redis", func(context.Context) error { return errors.New("connection refused") }),
	}

	// Act
	res := Evaluate(context.Background(), checkers)

	// Assert
	if res.Healthy {
		t.Fatal("expected unhealthy when a dependency fails")
	}
	if res.Checks["postgres"] != "ok" {
		t.Errorf("expected postgres ok, got %q", res.Checks["postgres"])
	}
	if !strings.HasPrefix(res.Checks["redis"], "error:") {
		t.Errorf("expected redis error prefix, got %q", res.Checks["redis"])
	}
}

func TestEvaluate_Timeout(t *testing.T) {
	// Arrange: a checker that outlives checkTimeout unless its context cancels.
	checkers := []Checker{
		NewChecker("slow", func(ctx context.Context) error {
			select {
			case <-time.After(10 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}),
	}

	// Act
	res := Evaluate(context.Background(), checkers)

	// Assert
	if res.Healthy {
		t.Fatal("expected unhealthy when a check exceeds the timeout")
	}
	if !strings.HasPrefix(res.Checks["slow"], "error:") {
		t.Errorf("expected slow error prefix, got %q", res.Checks["slow"])
	}
}
