package cached_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/cached"
)

func TestInRedis(t *testing.T) {
	addr, ok := os.LookupEnv("TEST_REDIS_ADDR_THAT_WILL_BE_WIPED")
	if !ok {
		t.Skip("set TEST_REDIS_ADDR_THAT_WILL_BE_WIPED and beware")
	}
	// addr = "localhost:6379"

	ctx := context.Background()
	ExerciseCache(t, func(tb testing.TB) cached.Cache[string, []byte] {
		c := cached.InRedis(addr)
		err := c.FlushDB(ctx)
		require.NoError(tb, err)
		return c
	}, "foo", []byte("bar"))
}
