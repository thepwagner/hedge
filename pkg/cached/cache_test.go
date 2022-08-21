package cached_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/cached"
)

func TestInMemory(t *testing.T) {
	ctx := context.Background()
	cache := cached.InMemory[string, string]()

	t.Run("not found", func(t *testing.T) {
		notFound, err := cache.Get(ctx, "not found")
		require.NoError(t, err)
		assert.Nil(t, notFound)
	})

	t.Run("roundtrip", func(t *testing.T) {
		err := cache.Set(ctx, "foo", "bar", time.Minute)
		require.NoError(t, err)

		stored, err := cache.Get(ctx, "foo")
		require.NoError(t, err)
		assert.Equal(t, "bar", *stored)
	})

	t.Run("respects TTL", func(t *testing.T) {
		err := cache.Set(ctx, "foo", "bar", -time.Minute)
		require.NoError(t, err)

		stored, err := cache.Get(ctx, "foo")
		require.NoError(t, err)
		assert.Nil(t, stored)
	})
}
