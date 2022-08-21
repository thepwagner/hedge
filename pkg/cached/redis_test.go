package cached_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/cached"
)

func TestInRedis(t *testing.T) {
	t.Skip("requires live redis server")

	storage := cached.InRedis("localhost:6379")
	ctx := context.Background()

	val := []byte("bar")
	err := storage.Set(ctx, "foo", val, time.Minute)
	require.NoError(t, err)

	cached, err := storage.Get(ctx, "foo")
	require.NoError(t, err)
	require.Equal(t, val, *cached)
}
