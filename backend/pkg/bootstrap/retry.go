// Package bootstrap holds startup helpers shared by the api and worker
// entrypoints. Retry tolerates dependencies (Postgres, Redis, Azurite) that are
// still warming up when the process starts — the docker-compose race the client
// warned about.
package bootstrap

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	maxAttempts = 15
	retryDelay  = 2 * time.Second
)

// Retry runs fn until it succeeds, the context is cancelled, or attempts are
// exhausted (~30s total). Each failed attempt is logged as a warning.
func Retry(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		log.Warn().
			Err(lastErr).
			Str("dependency", name).
			Int("attempt", attempt).
			Int("max", maxAttempts).
			Msg("dependency not ready, retrying")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
	}
	return lastErr
}
