package cached

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"time"
)

// WithGzip applys gzip compression to every value of a []byte-valued cache.
func WithGzip[K comparable, V ~[]byte](cache Cache[K, V]) *GzipStorage[K, V] {
	return &GzipStorage[K, V]{cache: cache}
}

type GzipStorage[K comparable, V ~[]byte] struct {
	cache Cache[K, V]
}

var _ Cache[string, []byte] = (*GzipStorage[string, []byte])(nil)

func (g GzipStorage[K, V]) Get(ctx context.Context, key K) (*V, error) {
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
	ret := V(decompressed)
	return &ret, nil
}

func (g GzipStorage[K, V]) Set(ctx context.Context, key K, value V, ttl time.Duration) error {
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
