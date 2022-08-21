package cached

import (
	"context"
	"encoding/json"
	"time"
)

type ByteStorage Cache[string, []byte]

// AsJSON caches a function result as JSON within byte storage.
func AsJSON[K comparable, V any](storage ByteStorage, ttl time.Duration, wrapped Function[K, V]) Function[K, V] {
	return func(ctx context.Context, arg K) (V, error) {
		var zero V
		keyB, err := json.Marshal(arg)
		if err != nil {
			return zero, err
		}
		key := string(keyB)

		if cached, err := storage.Get(ctx, key); err != nil {
			return zero, err
		} else if cached != nil {
			var ret V
			if err := json.Unmarshal(*cached, &ret); err != nil {
				return zero, err
			}
			return ret, nil
		}

		calculated, err := wrapped(ctx, arg)
		if err != nil {
			return zero, err
		}

		val, err := json.Marshal(calculated)
		if err != nil {
			return zero, err
		}
		if err := storage.Set(ctx, key, val, ttl); err != nil {
			return zero, err
		}
		return calculated, nil
	}
}
