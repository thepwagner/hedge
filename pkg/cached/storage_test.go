package cached_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/proto/hedge/v1"
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
	cachedExpensiveThing := cached.Wrap(fakeRedis, expensiveThing)

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		ret, err := cachedExpensiveThing(ctx, complexArgs{Foo: "foo", Bar: 1})
		require.NoError(t, err)
		assert.Equal(t, int64(1), ret.Baz)
		assert.Equal(t, int64(1), atomic.LoadInt64(&ctr))
	}
}

func TestAsProtoBuf(t *testing.T) {
	var ctr int64
	expensiveThing := func(_ context.Context, arg int) (*hedge.SignedEntry, error) {
		v := atomic.AddInt64(&ctr, 1)
		return &hedge.SignedEntry{KeyId: fmt.Sprintf("key%d", v)}, nil
	}

	fakeRedis := cached.InMemory[string, []byte]()
	cachedExpensiveThing := cached.Wrap(fakeRedis, expensiveThing, cached.AsProtoBuf[int, *hedge.SignedEntry]())

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		ret, err := cachedExpensiveThing(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, "key1", ret.KeyId)
		assert.Equal(t, int64(1), atomic.LoadInt64(&ctr))
	}
}
