package cache

import (
	"context"
	"encoding/json"
	"time"
)

type JSONCache[V any] struct {
	storage Storage
}

var _ Cache[string] = (*JSONCache[string])(nil)

func NewJSONCache[V any](storage Storage) JSONCache[V] {
	return JSONCache[V]{storage: storage}
}

func (c JSONCache[V]) Get(ctx context.Context, key string) (V, error) {
	var value V
	b, err := c.storage.Get(ctx, key)
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(b, &value); err != nil {
		return value, err
	}
	return value, nil
}

func (c JSONCache[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.storage.Set(ctx, key, b, ttl)
}
