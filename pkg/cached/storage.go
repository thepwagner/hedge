package cached

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"reflect"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

type ByteStorage Cache[string, []byte]

type KeyMapper[K any] func(K) (string, error)

type MappingOptions[K any, V any] struct {
	tracer       trace.Tracer
	spanName     string
	KeyMapper    KeyMapper[K]
	ValueToBytes func(context.Context, V) ([]byte, error)
	BytesToValue func(context.Context, []byte) (V, error)
	TTL          time.Duration
}

type MappingOption[K any, V any] func(*MappingOptions[K, V])

func Wrap[K any, V any](storage ByteStorage, wrapped Function[K, V], opts ...MappingOption[K, V]) Function[K, V] {
	var mappingOpt MappingOptions[K, V]
	for _, opt := range opts {
		opt(&mappingOpt)
	}

	if mappingOpt.KeyMapper == nil {
		mappingOpt.KeyMapper = HashingKeyMapper[K](sha256.New)
	}
	if mappingOpt.BytesToValue == nil || mappingOpt.ValueToBytes == nil {
		AsJSON[K, V]()(&mappingOpt)
	}
	if mappingOpt.TTL == 0 {
		mappingOpt.TTL = 5 * time.Minute
	}

	if mappingOpt.tracer != nil {
		btv := mappingOpt.BytesToValue
		vtb := mappingOpt.ValueToBytes
		mappingOpt.BytesToValue = func(ctx context.Context, b []byte) (V, error) {
			ctx, span := mappingOpt.tracer.Start(ctx, fmt.Sprintf("cache.%s.BytesToValue", mappingOpt.spanName))
			defer span.End()
			v, err := btv(ctx, b)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return v, err
			}
			return v, nil
		}
		mappingOpt.ValueToBytes = func(ctx context.Context, v V) ([]byte, error) {
			ctx, span := mappingOpt.tracer.Start(ctx, fmt.Sprintf("cache.%s.ValueToBytes", mappingOpt.spanName))
			defer span.End()
			b, err := vtb(ctx, v)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return nil, err
			}
			return b, nil
		}
	}

	return func(ctx context.Context, arg K) (V, error) {
		if mappingOpt.tracer != nil {
			var span trace.Span
			ctx, span = mappingOpt.tracer.Start(ctx, fmt.Sprintf("cache.%s", mappingOpt.spanName))
			defer span.End()
		}

		var zero V
		key, err := mappingOpt.KeyMapper(arg)
		if err != nil {
			return zero, err
		}

		if cached, err := storage.Get(ctx, key); err != nil {
			return zero, err
		} else if cached != nil {
			return mappingOpt.BytesToValue(ctx, *cached)
		}

		calculated, err := wrapped(ctx, arg)
		if err != nil {
			return zero, err
		}

		b, err := mappingOpt.ValueToBytes(ctx, calculated)
		if err != nil {
			return zero, err
		}
		if err := storage.Set(ctx, key, b, mappingOpt.TTL); err != nil {
			return zero, err
		}
		return calculated, nil
	}
}

func HashingKeyMapper[K any](hasher func() hash.Hash) KeyMapper[K] {
	return func(k K) (string, error) {
		h := hasher()
		if err := json.NewEncoder(h).Encode(k); err != nil {
			return "", err
		}
		return fmt.Sprintf("%x", h.Sum(nil)), nil
	}
}

func AsJSON[K any, V any]() MappingOption[K, V] {
	return func(opt *MappingOptions[K, V]) {
		opt.BytesToValue = func(_ context.Context, b []byte) (V, error) {
			var v V
			if err := json.Unmarshal(b, &v); err != nil {
				return v, fmt.Errorf("decoding json: %w", err)
			}
			return v, nil
		}
		opt.ValueToBytes = func(_ context.Context, v V) ([]byte, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("encoding json: %w", err)
			}
			return b, nil
		}
	}
}

func AsProtoBuf[K any, V proto.Message]() MappingOption[K, V] {
	var v V
	vType := reflect.TypeOf(v).Elem()
	return func(opt *MappingOptions[K, V]) {
		opt.BytesToValue = func(_ context.Context, b []byte) (V, error) {
			ret := reflect.New(vType).Interface().(V)
			if err := proto.Unmarshal(b, ret); err != nil {
				return v, fmt.Errorf("decoding json: %w", err)
			}
			return ret, nil
		}
		opt.ValueToBytes = func(_ context.Context, v V) ([]byte, error) {
			b, err := proto.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("encoding json: %w", err)
			}
			return b, nil
		}
	}
}

func WithTTL[K any, V any](ttl time.Duration) MappingOption[K, V] {
	return func(opt *MappingOptions[K, V]) {
		opt.TTL = ttl
	}
}

func WithMappingTracer[K any, V any](tracer trace.Tracer, spanName string) MappingOption[K, V] {
	return func(opt *MappingOptions[K, V]) {
		opt.tracer = tracer
		opt.spanName = spanName
	}
}
