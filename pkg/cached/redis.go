package cached

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/extra/redisotel/v9"
	"github.com/go-redis/redis/v9"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
)

// Redis is shared external ByteStorage.
type Redis struct {
	redis *redis.Client
}

var _ ByteStorage = (*Redis)(nil)

// InRedis returns Redis-backed ByteStorage.
func InRedis(addr string, tp trace.TracerProvider) *Redis {
	redisC := redis.NewClient(&redis.Options{
		Addr:         addr,
		MinIdleConns: 5,
	})

	redisC.AddHook(redisotel.NewTracingHook(redisotel.WithTracerProvider(tp), redisotel.WithAttributes(semconv.NetPeerNameKey.String(addr))))
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
