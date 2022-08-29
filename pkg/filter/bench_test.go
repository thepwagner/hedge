package filter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/filter"
	"github.com/thepwagner/hedge/proto/hedge/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func BenchmarkCue(b *testing.B) {
	ctx := context.Background()
	pkg := &TestPackage{Name: "foo"}

	pred, err := filter.MatchesCue("testdata/name_foo.cue")
	require.NoError(b, err)

	for i := 0; i < b.N; i++ {
		d, err := json.Marshal(pkg)
		require.NoError(b, err)
		ok, err := pred(ctx, d)
		require.NoError(b, err)
		assert.True(b, ok)
	}
}

func BenchmarkRego(b *testing.B) {
	ctx := context.Background()
	pkg := &TestPackage{Name: "foo"}

	pred, err := filter.MatchesRego[*TestPackage](ctx, "testdata/name_foo.rego")
	require.NoError(b, err)

	for i := 0; i < b.N; i++ {
		ok, err := pred(ctx, pkg)
		require.NoError(b, err)
		assert.True(b, ok)
	}
}

func BenchmarkNative(b *testing.B) {
	ctx := context.Background()
	pkg := &TestPackage{Name: "foo"}

	signers := map[string]struct{}{
		"key1": {},
		"key2": {},
	}

	pred := func(_ context.Context, pkg *TestPackage) (bool, error) {
		if pkg == nil {
			return false, nil
		}
		if pkg.Name != "foo" {
			return false, nil
		}
		if pkg.Deprecated {
			return false, nil
		}

		if sig := pkg.Signature; sig != nil {
			_, ok := signers[sig.KeyFingerprint]
			return ok, nil
		}
		return true, nil
	}

	for i := 0; i < b.N; i++ {
		ok, err := pred(ctx, pkg)
		require.NoError(b, err)
		assert.True(b, ok)
	}
}

func BenchmarkMappyMcMapface(b *testing.B) {
	ctx := context.Background()
	pkg := &hedge.DebianPackage{
		Name: "foo",
	}

	// signers := map[string]struct{}{
	// 	"key1": {},
	// 	"key2": {},
	// }

	pred := func(_ context.Context, pkg map[string]interface{}) (bool, error) {
		if pkg == nil {
			return false, nil
		}
		if pkg["name"] != "foo" {
			return false, nil
		}
		if dep, ok := pkg["deprecated"].(bool); ok && dep {
			return false, nil
		}

		// if sig := pkg.Signature; sig != nil {
		// 	_, ok := signers[sig.KeyFingerprint]
		// 	return ok, nil
		// }
		return true, nil
	}

	m := map[string]interface{}{}
	pkg.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		m[string(fd.Name())] = v.Interface()
		return true
	})

	for i := 0; i < b.N; i++ {

		ok, err := pred(ctx, m)
		require.NoError(b, err)
		assert.True(b, ok)
	}
}
