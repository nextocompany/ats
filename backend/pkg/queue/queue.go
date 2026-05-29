// Package queue wraps hibiken/asynq for the AI processing pipeline. It derives
// asynq's RedisClientOpt from the same REDIS_URL the rest of the app uses, so
// Sprint 1 introduces no new infrastructure.
package queue

import (
	"fmt"

	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"
)

// RedisOpt converts a redis:// URL into asynq's RedisClientOpt.
func RedisOpt(url string) (asynq.RedisClientOpt, error) {
	parsed, err := goredis.ParseURL(url)
	if err != nil {
		return asynq.RedisClientOpt{}, fmt.Errorf("queue: parse redis url: %w", err)
	}
	return asynq.RedisClientOpt{
		Addr:     parsed.Addr,
		Username: parsed.Username,
		Password: parsed.Password,
		DB:       parsed.DB,
	}, nil
}

// NewClient builds an asynq client for enqueuing tasks.
func NewClient(url string) (*asynq.Client, error) {
	opt, err := RedisOpt(url)
	if err != nil {
		return nil, err
	}
	return asynq.NewClient(opt), nil
}

// NewInspector builds an asynq inspector for querying task state.
func NewInspector(url string) (*asynq.Inspector, error) {
	opt, err := RedisOpt(url)
	if err != nil {
		return nil, err
	}
	return asynq.NewInspector(opt), nil
}
