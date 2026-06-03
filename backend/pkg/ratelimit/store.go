// Package ratelimit adapts the shared go-redis client to fiber.Storage so the
// public-API limiter counts requests cluster-wide instead of per process.
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// keyPrefix namespaces limiter keys so they never collide with asynq's keys in
// the shared Redis database.
const keyPrefix = "ratelimit:"

// opTimeout bounds each Redis call; on timeout/error the store fails OPEN (the
// limiter treats it as a miss) so a Redis blip never blocks the public flow.
const opTimeout = 3 * time.Second

// scanBatch is the SCAN page size / DEL batch size used by Reset.
const scanBatch = 100

// RedisStore implements fiber.Storage over a borrowed *goredis.Client. It does
// NOT own the client — Close is a no-op; the api owns the client lifecycle.
type RedisStore struct{ client *goredis.Client }

// New builds a RedisStore over an existing client.
func New(client *goredis.Client) *RedisStore { return &RedisStore{client: client} }

// Get returns the value for key, or (nil, nil) on miss. A Redis error fails open
// (also (nil, nil)) so the limiter never 500s on a transient outage.
func (s *RedisStore) Get(key string) ([]byte, error) {
	if key == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	val, err := s.client.Get(ctx, keyPrefix+key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, nil // miss
	}
	if err != nil {
		log.Warn().Err(err).Msg("ratelimit: get failed; failing open")
		return nil, nil // fail open
	}
	return val, nil
}

// Set stores val for key with the given expiration (0 = no expiry). Empty key or
// value is ignored per the fiber.Storage contract. A Redis error is logged but
// never propagated — the limiter must not 500 on a Redis blip.
func (s *RedisStore) Set(key string, val []byte, exp time.Duration) error {
	if key == "" || len(val) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	if err := s.client.Set(ctx, keyPrefix+key, val, exp).Err(); err != nil {
		log.Warn().Err(err).Msg("ratelimit: set failed; failing open")
	}
	return nil
}

// Delete removes key. A Redis error is logged, not propagated.
func (s *RedisStore) Delete(key string) error {
	if key == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	if err := s.client.Del(ctx, keyPrefix+key).Err(); err != nil {
		log.Warn().Err(err).Msg("ratelimit: delete failed")
	}
	return nil
}

// Reset deletes ONLY ratelimit:* keys. It MUST NOT FLUSHDB — the same Redis holds
// the asynq job queue. SCAN avoids blocking Redis on a large keyspace.
func (s *RedisStore) Reset() error {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	iter := s.client.Scan(ctx, 0, keyPrefix+"*", scanBatch).Iterator()
	var batch []string
	for iter.Next(ctx) {
		batch = append(batch, iter.Val())
		if len(batch) >= scanBatch {
			if err := s.client.Del(ctx, batch...).Err(); err != nil {
				return fmt.Errorf("ratelimit: reset del: %w", err)
			}
			batch = batch[:0]
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("ratelimit: reset scan: %w", err)
	}
	if len(batch) > 0 {
		if err := s.client.Del(ctx, batch...).Err(); err != nil {
			return fmt.Errorf("ratelimit: reset del: %w", err)
		}
	}
	return nil
}

// Close is a no-op: the client is owned by the api process, not this adapter.
func (s *RedisStore) Close() error { return nil }
