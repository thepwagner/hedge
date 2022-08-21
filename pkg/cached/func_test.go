package cached_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/cached"
)

func TestCached(t *testing.T) {
	// Function with a side effect, so we can track when it is invoked:
	var ctr int64
	expensiveThing := func(_ context.Context, delta int64) (int64, error) {
		return atomic.AddInt64(&ctr, delta), nil
	}

	intCache := cached.InMemory[int64, int64]()
	cachedExpensiveThing := cached.Cached[int64, int64](intCache, time.Minute, expensiveThing)

	// Multiple calls with the same argument return the same result:
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		ret, err := cachedExpensiveThing(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), ret)
		assert.Equal(t, int64(1), atomic.LoadInt64(&ctr))
	}

	// Different arguments are passed through
	for i := 0; i < 5; i++ {
		ret, err := cachedExpensiveThing(ctx, 2)
		require.NoError(t, err)
		assert.Equal(t, int64(1+2), ret)
		assert.Equal(t, int64(1+2), atomic.LoadInt64(&ctr))
	}
}
