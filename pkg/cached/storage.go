package cached

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"reflect"
	"time"

	"google.golang.org/protobuf/proto"
)

type ByteStorage Cache[string, []byte]

type KeyMapper[K any] func(K) (string, error)

type MappingOptions[K any, V any] struct {
	KeyMapper    KeyMapper[K]
	ValueToBytes func(V) ([]byte, error)
	BytesToValue func([]byte) (V, error)
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

	return func(ctx context.Context, arg K) (V, error) {
		var zero V
		key, err := mappingOpt.KeyMapper(arg)
		if err != nil {
			return zero, err
		}

		if cached, err := storage.Get(ctx, key); err != nil {
			return zero, err
		} else if cached != nil {
			return mappingOpt.BytesToValue(*cached)
		}

		calculated, err := wrapped(ctx, arg)
		if err != nil {
			return zero, err
		}

		b, err := mappingOpt.ValueToBytes(calculated)
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
		opt.BytesToValue = func(b []byte) (V, error) {
			var v V
			if err := json.Unmarshal(b, &v); err != nil {
				return v, fmt.Errorf("decoding json: %w", err)
			}
			return v, nil
		}
		opt.ValueToBytes = func(v V) ([]byte, error) {
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
		opt.BytesToValue = func(b []byte) (V, error) {
			ret := reflect.New(vType).Interface().(V)
			if err := proto.Unmarshal(b, ret); err != nil {
				return v, fmt.Errorf("decoding json: %w", err)
			}
			return ret, nil
		}
		opt.ValueToBytes = func(v V) ([]byte, error) {
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
