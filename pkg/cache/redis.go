package cache

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v9"
)

type Redis struct {
	redis *redis.Client
}

var _ Storage = (*Redis)(nil)

func NewRedis(addr string) *Redis {
	redisC := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &Redis{redis: redisC}
}

func (r *Redis) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := r.redis.Get(ctx, key).Bytes()
	if err == nil {
		return b, nil
	}
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	return nil, err
}

func (r *Redis) Set(ctx context.Context, key string, b []byte, ttl time.Duration) error {
	return r.redis.Set(ctx, key, b, ttl).Err()
}
