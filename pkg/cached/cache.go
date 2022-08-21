package cached

import (
	"context"
	"sync"
	"time"
)

// Cache assocaites keys and values.
type Cache[K comparable, V any] interface {
	// Get returns (nil, nil) when the value is not found.
	Get(ctx context.Context, key K) (*V, error)

	// Set stores a value.
	Set(ctx context.Context, key K, value V, ttl time.Duration) error
}

type InMemoryCache[K comparable, V any] struct {
	now func() time.Time

	mu   sync.RWMutex
	data map[K]memoryCacheEntry[V]
}

type memoryCacheEntry[V any] struct {
	value  *V
	expiry time.Time
}

func InMemory[K comparable, V any]() *InMemoryCache[K, V] {
	return &InMemoryCache[K, V]{
		data: make(map[K]memoryCacheEntry[V]),
		now:  time.Now,
	}
}

func (b *InMemoryCache[K, V]) Get(ctx context.Context, key K) (*V, error) {
	b.mu.RLock()
	v, ok := b.data[key]
	b.mu.RUnlock()

	if !ok {
		return nil, nil
	}
	if b.now().After(v.expiry) {
		b.delete(key)
		return nil, nil
	}
	return v.value, nil
}

func (b *InMemoryCache[K, V]) delete(key K) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if v, ok := b.data[key]; !ok {
		// already deleted
		return
	} else if !b.now().After(v.expiry) {
		// refreshed
		return
	}
	delete(b.data, key)
}

func (b *InMemoryCache[K, V]) Set(ctx context.Context, key K, value V, ttl time.Duration) error {
	expiry := b.now().Add(ttl)

	b.mu.Lock()
	b.data[key] = memoryCacheEntry[V]{
		value:  &value,
		expiry: expiry,
	}
	b.mu.Unlock()
	return nil
}
