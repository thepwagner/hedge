package cached

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"time"
)

// WithGzip applys gzip compression to every value of a []byte-valued cache.
func WithGzip[K comparable](cache Cache[K, []byte]) *GzipStorage[K] {
	return &GzipStorage[K]{cache: cache}
}

type GzipStorage[K comparable] struct {
	cache Cache[K, []byte]
}

var _ Cache[string, []byte] = (*GzipStorage[string])(nil)

func (g GzipStorage[K]) Get(ctx context.Context, key K) (*[]byte, error) {
	b, err := g.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, nil
	}

	gz, err := gzip.NewReader(bytes.NewReader(*b))
	if err != nil {
		return nil, err
	}
	decompressed, err := io.ReadAll(gz)
	if err != nil {
		return nil, err
	}
	return &decompressed, nil
}

func (g GzipStorage[K]) Set(ctx context.Context, key K, value []byte, ttl time.Duration) error {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(value); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return g.cache.Set(ctx, key, buf.Bytes(), ttl)
}
