package cached

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

type ByteStorage Cache[string, []byte]

type MappingOptions[K comparable, V any] struct {
	KeyMapper func(K) (string, error)
}

// AsJSON caches a function result as JSON within byte storage.
func AsJSON[K comparable, V any](tracer trace.Tracer, storage ByteStorage, ttl time.Duration, wrapped Function[K, V]) Function[K, V] {
	opts := MappingOptions[K, V]{
		KeyMapper: func(k K) (string, error) {
			h := sha256.New()
			if err := json.NewEncoder(h).Encode(k); err != nil {
				return "", err
			}
			return fmt.Sprintf("%x", h.Sum(nil)), nil
		},
	}

	return func(ctx context.Context, arg K) (V, error) {
		_, keyMapSpan := tracer.Start(ctx, "keyMapper")
		var zero V
		key, err := opts.KeyMapper(arg)
		if err != nil {
			return zero, err
		}
		keyMapSpan.End()

		if cached, err := storage.Get(ctx, key); err != nil {
			return zero, err
		} else if cached != nil {
			_, unmarshalSpan := tracer.Start(ctx, "unmarshal")
			var ret V
			if err := json.Unmarshal(*cached, &ret); err != nil {
				return zero, err
			}
			unmarshalSpan.End()
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

func AsProtoBuf[K comparable, V proto.Message](tracer trace.Tracer, storage ByteStorage, ttl time.Duration, wrapped Function[K, V]) Function[K, V] {
	opts := MappingOptions[K, V]{
		KeyMapper: func(k K) (string, error) {
			h := sha256.New()
			if err := json.NewEncoder(h).Encode(k); err != nil {
				return "", err
			}
			return fmt.Sprintf("%x", h.Sum(nil)), nil
		},
	}

	return func(ctx context.Context, arg K) (V, error) {
		_, keyMapSpan := tracer.Start(ctx, "keyMapper")
		var zero V
		key, err := opts.KeyMapper(arg)
		if err != nil {
			return zero, err
		}
		keyMapSpan.End()

		if cached, err := storage.Get(ctx, key); err != nil {
			return zero, err
		} else if cached != nil {
			_, unmarshalSpan := tracer.Start(ctx, "unmarshal")
			var ret V
			ret = ret.ProtoReflect().New().Interface().(V)
			if err := proto.Unmarshal(*cached, ret); err != nil {
				return zero, err
			}
			unmarshalSpan.End()
			return ret, nil
		}

		calculated, err := wrapped(ctx, arg)
		if err != nil {
			return zero, err
		}

		val, err := proto.Marshal(calculated)
		if err != nil {
			return zero, err
		}
		if err := storage.Set(ctx, key, val, ttl); err != nil {
			return zero, err
		}
		return calculated, nil
	}
}
