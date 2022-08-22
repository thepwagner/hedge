package cached_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/cached"
)

func TestWithPrefix(t *testing.T) {
	ctx := context.Background()
	storage := cached.InMemory[string, string]()
	prefixed := cached.WithPrefix[string, string]("test", storage)

	err := prefixed.Set(ctx, "foo", "bar", time.Minute)
	require.NoError(t, err)

	stored, err := prefixed.Get(ctx, "foo")
	require.NoError(t, err)
	require.Equal(t, "bar", *stored)

	notFound, err := storage.Get(ctx, "foo")
	require.NoError(t, err)
	assert.Nil(t, notFound)

	fromStorage, err := storage.Get(ctx, "test:foo")
	require.NoError(t, err)
	require.Equal(t, "bar", *fromStorage)
}

func TestWithPrefix_Custom(t *testing.T) {
	ctx := context.Background()

	type customKey string
	storage := cached.InMemory[customKey, string]()
	prefixed := cached.WithPrefix[customKey, string]("test", storage)

	err := prefixed.Set(ctx, "foo", "bar", time.Minute)
	require.NoError(t, err)
	stored, err := prefixed.Get(ctx, "foo")
	require.NoError(t, err)
	require.Equal(t, "bar", *stored)
}
