// Package redis owns the Redis client used for caching and (in Sprint 1+) the
// job queue. Sprint 0 only establishes connectivity.
package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

// Connect parses the URL, creates a client, and verifies connectivity with a
// ping. The caller owns the returned client and must Close it on shutdown.
func Connect(ctx context.Context, url string) (*goredis.Client, error) {
	opts, err := goredis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis: parse url: %w", err)
	}
	client := goredis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	return client, nil
}
