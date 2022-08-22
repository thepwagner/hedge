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
	t.Run("string,string", func(t *testing.T) {
		t.Parallel()
		ExerciseCache(t, func(testing.TB) cached.Cache[string, string] { return cached.InMemory[string, string]() }, "foo", "bar")
	})
	t.Run("int,int", func(t *testing.T) {
		t.Parallel()
		ExerciseCache(t, func(testing.TB) cached.Cache[int64, int64] { return cached.InMemory[int64, int64]() }, 321, 123)
	})
}

func ExerciseCache[K comparable, V any](t *testing.T, factory func(testing.TB) cached.Cache[K, V], key K, value V) {
	t.Helper()
	ctx := context.Background()

	t.Run("get not found", func(t *testing.T) {
		cache := factory(t)
		notFound, err := cache.Get(ctx, key)
		require.NoError(t, err)
		assert.Nil(t, notFound)
	})

	t.Run("set", func(t *testing.T) {
		cache := factory(t)
		err := cache.Set(ctx, key, value, time.Minute)
		assert.NoError(t, err)
	})

	t.Run("get after set", func(t *testing.T) {
		cache := factory(t)
		err := cache.Set(ctx, key, value, time.Minute)
		require.NoError(t, err)

		stored, err := cache.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value, *stored)
	})

	t.Run("get after expiry", func(t *testing.T) {
		cache := factory(t)

		err := cache.Set(ctx, key, value, 100*time.Millisecond)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		stored, err := cache.Get(ctx, key)
		require.NoError(t, err)
		assert.Nil(t, stored)
	})
}
