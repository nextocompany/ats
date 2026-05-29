// Package health aggregates dependency probes (Postgres, Redis, Blob) into a
// single endpoint. The endpoint returns 200 only when every dependency is
// reachable, so a green /health proves the service is correctly wired.
package health

import (
	"context"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/nexto/hr-ats/pkg/httpx"
)

const checkTimeout = 2 * time.Second

// Checker probes a single backing dependency.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

type checkerFunc struct {
	name string
	fn   func(ctx context.Context) error
}

func (c checkerFunc) Name() string                    { return c.name }
func (c checkerFunc) Check(ctx context.Context) error { return c.fn(ctx) }

// NewChecker adapts a named function into a Checker.
func NewChecker(name string, fn func(ctx context.Context) error) Checker {
	return checkerFunc{name: name, fn: fn}
}

// Result is the aggregate outcome of all checks. Healthy is excluded from JSON;
// the HTTP status conveys it. Checks maps dependency name to "ok" or "error: …".
type Result struct {
	Healthy bool              `json:"-"`
	Checks  map[string]string `json:"checks"`
}

// Evaluate runs all checkers concurrently, each bounded by checkTimeout, so a
// single slow dependency cannot stall the probe.
func Evaluate(ctx context.Context, checkers []Checker) Result {
	res := Result{Healthy: true, Checks: make(map[string]string, len(checkers))}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, checker := range checkers {
		wg.Add(1)
		go func(checker Checker) {
			defer wg.Done()

			cctx, cancel := context.WithTimeout(ctx, checkTimeout)
			defer cancel()
			err := checker.Check(cctx)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				res.Healthy = false
				res.Checks[checker.Name()] = "error: " + err.Error()
				return
			}
			res.Checks[checker.Name()] = "ok"
		}(checker)
	}
	wg.Wait()

	return res
}

// Handler returns a Fiber handler that reports aggregate health. It responds
// 200 when all checks pass and 503 (with the failing checks shown) otherwise.
func Handler(checkers ...Checker) fiber.Handler {
	return func(c *fiber.Ctx) error {
		res := Evaluate(c.UserContext(), checkers)

		env := httpx.Envelope[Result]{Success: res.Healthy, Data: res}
		status := fiber.StatusOK
		if !res.Healthy {
			status = fiber.StatusServiceUnavailable
			env.Error = "one or more dependencies unavailable"
		}
		return c.Status(status).JSON(env)
	}
}
