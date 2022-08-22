package cached

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v9"
)

// Redis is shared external ByteStorage.
type Redis struct {
	redis *redis.Client
}

var _ ByteStorage = (*Redis)(nil)

// InRedis returns Redis-backed ByteStorage.
func InRedis(addr string) *Redis {
	redisC := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &Redis{redis: redisC}
}

func (r *Redis) Get(ctx context.Context, key string) (*[]byte, error) {
	b, err := r.redis.Get(ctx, key).Bytes()
	if err == nil {
		return &b, nil
	}
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return nil, err
}

func (r *Redis) Set(ctx context.Context, key string, b []byte, ttl time.Duration) error {
	return r.redis.Set(ctx, key, b, durationFromContext(ctx, ttl)).Err()
}

func (r *Redis) FlushDB(ctx context.Context) error {
	return r.redis.FlushDB(ctx).Err()
}
