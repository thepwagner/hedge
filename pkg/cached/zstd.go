package cached

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/klauspost/compress/zstd"
)

type ZstdStorage[K comparable, V ~[]byte] struct {
	cache Cache[K, V]
}

var _ Cache[string, []byte] = (*ZstdStorage[string, []byte])(nil)

func WithZstd[K comparable, V ~[]byte](cache Cache[K, V]) *ZstdStorage[K, V] {
	return &ZstdStorage[K, V]{cache: cache}
}

func (g ZstdStorage[K, V]) Get(ctx context.Context, key K) (*V, error) {
	b, err := g.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, nil
	}

	gz, err := zstd.NewReader(bytes.NewReader(*b))
	if err != nil {
		return nil, err
	}
	decompressed, err := io.ReadAll(gz)
	if err != nil {
		return nil, err
	}
	ret := V(decompressed)
	return &ret, nil
}

func (g ZstdStorage[K, V]) Set(ctx context.Context, key K, value V, ttl time.Duration) error {
	var buf bytes.Buffer
	gz, err := zstd.NewWriter(&buf)
	if err != nil {
		return err
	}
	if _, err := gz.Write(value); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return g.cache.Set(ctx, key, buf.Bytes(), ttl)
}
