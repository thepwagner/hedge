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

type complexArgs struct {
	Foo string `json:"foo"`
	Bar int64  `json:"bar"`
}

type complexRet struct {
	Baz int64 `json:"baz"`
}

func TestAsJSON(t *testing.T) {
	// Function with a side effect, so we can track when it is invoked:
	var ctr int64
	expensiveThing := func(_ context.Context, arg complexArgs) (complexRet, error) {
		v := atomic.AddInt64(&ctr, arg.Bar)
		return complexRet{Baz: v}, nil
	}

	fakeRedis := cached.InMemory[string, []byte]()
	cachedExpensiveThing := cached.AsJSON(fakeRedis, time.Minute, expensiveThing)

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		ret, err := cachedExpensiveThing(ctx, complexArgs{Foo: "foo", Bar: 1})
		require.NoError(t, err)
		assert.Equal(t, int64(1), ret.Baz)
		assert.Equal(t, int64(1), atomic.LoadInt64(&ctr))
	}
}
