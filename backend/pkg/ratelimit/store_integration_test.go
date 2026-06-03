//go:build integration

package ratelimit

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	goredis "github.com/redis/go-redis/v9"
)

func redisURL() string {
	if v := os.Getenv("REDIS_URL"); v != "" {
		return v
	}
	return "redis://localhost:6379"
}

func newClient(t *testing.T) *goredis.Client {
	t.Helper()
	opts, err := goredis.ParseURL(redisURL())
	if err != nil {
		t.Fatalf("parse redis url: %v", err)
	}
	c := goredis.NewClient(opts)
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("redis ping (stack up?): %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestRedisStore_SetGetExpiryDelete(t *testing.T) {
	s := New(newClient(t))
	if err := s.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}

	// Set + Get round-trip.
	if err := s.Set("ip1", []byte("x"), 2*time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := s.Get("ip1")
	if err != nil || string(got) != "x" {
		t.Fatalf("Get(ip1) = (%q, %v), want (\"x\", nil)", got, err)
	}

	// Miss on an absent key.
	if got, err := s.Get("absent"); err != nil || got != nil {
		t.Errorf("Get(absent) = (%v, %v), want (nil, nil)", got, err)
	}

	// TTL expiry frees the window.
	time.Sleep(2500 * time.Millisecond)
	if got, err := s.Get("ip1"); err != nil || got != nil {
		t.Errorf("after TTL Get(ip1) = (%v, %v), want (nil, nil)", got, err)
	}

	// Delete.
	if err := s.Set("ip2", []byte("y"), time.Minute); err != nil {
		t.Fatalf("set ip2: %v", err)
	}
	if err := s.Delete("ip2"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, err := s.Get("ip2"); err != nil || got != nil {
		t.Errorf("after Delete Get(ip2) = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestRedisStore_ResetScopedToPrefix(t *testing.T) {
	client := newClient(t)
	s := New(client)
	ctx := context.Background()

	// An unrelated key (mimics asynq's namespace) must survive Reset.
	if err := client.Set(ctx, "asynq:fake-queue", "job", time.Minute).Err(); err != nil {
		t.Fatalf("seed asynq key: %v", err)
	}
	t.Cleanup(func() { _ = client.Del(ctx, "asynq:fake-queue").Err() })

	if err := s.Set("ipX", []byte("z"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := s.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}

	// ratelimit:* gone...
	if got, err := s.Get("ipX"); err != nil || got != nil {
		t.Errorf("after Reset Get(ipX) = (%v, %v), want (nil, nil)", got, err)
	}
	// ...asynq key untouched.
	if v, err := client.Get(ctx, "asynq:fake-queue").Result(); err != nil || v != "job" {
		t.Errorf("Reset wiped a non-ratelimit key: got (%q, %v), want (\"job\", nil)", v, err)
	}
}

// TestLimiter_SharedAcrossInstances proves the window is shared via Redis: two
// independent fiber apps (standing in for two api replicas) backed by the same
// store enforce a single combined budget per key.
func TestLimiter_SharedAcrossInstances(t *testing.T) {
	store := New(newClient(t))
	if err := store.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}

	const key = "10.0.0.99"
	build := func() *fiber.App {
		app := fiber.New()
		app.Use(limiter.New(limiter.Config{
			Max:          3,
			Expiration:   time.Minute,
			Storage:      store,
			KeyGenerator: func(*fiber.Ctx) string { return key },
			LimitReached: func(c *fiber.Ctx) error {
				return fiber.NewError(fiber.StatusTooManyRequests, "rate limit exceeded")
			},
		}))
		app.Get("/p", func(c *fiber.Ctx) error { return c.SendString("ok") })
		return app
	}
	appA, appB := build(), build()

	hit := func(app *fiber.App) int {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/p", nil))
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		return resp.StatusCode
	}

	// Budget = 3: 2 via replica A + 1 via replica B → all allowed.
	if c := hit(appA); c != fiber.StatusOK {
		t.Fatalf("A req1 = %d, want 200", c)
	}
	if c := hit(appA); c != fiber.StatusOK {
		t.Fatalf("A req2 = %d, want 200", c)
	}
	if c := hit(appB); c != fiber.StatusOK {
		t.Fatalf("B req3 = %d, want 200", c)
	}
	// 4th request on either replica exceeds the SHARED window.
	if c := hit(appB); c != fiber.StatusTooManyRequests {
		t.Fatalf("B req4 = %d, want 429 (window not shared across instances?)", c)
	}
	if c := hit(appA); c != fiber.StatusTooManyRequests {
		t.Fatalf("A req5 = %d, want 429", c)
	}

	_ = store.Reset()
}
