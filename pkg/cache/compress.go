package cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"time"
)

type GzipStorage struct {
	storage Storage
}

var _ Storage = (*GzipStorage)(nil)

func NewGzipStorage(storage Storage) GzipStorage {
	return GzipStorage{storage: storage}
}

func (g GzipStorage) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := g.storage.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(gz)
}

func (g GzipStorage) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(value); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return g.storage.Set(ctx, key, buf.Bytes(), ttl)
}
