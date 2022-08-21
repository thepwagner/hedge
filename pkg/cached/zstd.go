package cached

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/klauspost/compress/zstd"
)

type ZstdStorage[K comparable] struct {
	cache Cache[K, []byte]
}

var _ Cache[string, []byte] = (*ZstdStorage[string])(nil)

func WithZstd[K comparable](cache Cache[K, []byte]) *ZstdStorage[K] {
	return &ZstdStorage[K]{cache: cache}
}

func (g ZstdStorage[K]) Get(ctx context.Context, key K) (*[]byte, error) {
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
	return &decompressed, nil
}

func (g ZstdStorage[K]) Set(ctx context.Context, key K, value []byte, ttl time.Duration) error {
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
