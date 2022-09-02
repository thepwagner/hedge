package debian_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/pkg/registry/debian"
	"github.com/thepwagner/hedge/proto/hedge/v1"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func TestRemoteLoader(t *testing.T) {
	ctx := context.Background()
	tp := observability.TestTracerProvider(t)
	tracer := tp.Tracer("")

	ctx, span := tracer.Start(ctx, "TestRemoteLoader")
	defer span.End()
	fmt.Printf("http://riker.pwagner.net:16686/trace/%s\n", span.SpanContext().TraceID())

	key, err := os.ReadFile("testdata/bullseye_pubkey.txt")
	require.NoError(t, err)

	storage := cached.InRedis("localhost:6379", tp)
	fetch := cached.URLFetcher(&http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport, otelhttp.WithTracerProvider(tp)),
	})
	fetch = cached.Wrap(cached.WithPrefix[string, []byte]("debian_urls", storage), fetch)

	t.Run("NewReleaseLoader", func(t *testing.T) {
		ctx, span := tracer.Start(ctx, "NewReleaseLoader")
		defer span.End()

		releases := debian.NewReleaseLoader2(tracer, fetch)
		loader := cached.Race(tracer, "load release", map[string]cached.Function[debian.LoadReleaseArgs, *hedge.DebianRelease]{
			"direct": releases.Load,
			"cached": cached.Wrap(storage, releases.Load, cached.AsProtoBuf[debian.LoadReleaseArgs, *hedge.DebianRelease]()),
		})

		release, err := loader(ctx, debian.LoadReleaseArgs{
			MirrorURL:  "https://debian.mirror.rafal.ca/debian/",
			SigningKey: string(key),
			Dist:       "bullseye",
		})
		require.NoError(t, err)

		assert.True(t, release.AcquireByHash)
		assert.Equal(t, []string{
			"all", "amd64", "arm64", "armel", "armhf", "i386", "mips64el", "mipsel", "ppc64el", "s390x",
		}, release.Architectures)
		assert.Equal(t, "https://metadata.ftp-master.debian.org/changelogs/@CHANGEPATH@_changelog", release.Changelogs)
		assert.Equal(t, "bullseye", release.Codename)
		assert.Equal(t, []string{"main", "contrib", "non-free"}, release.Components)
		assert.False(t, release.Date.AsTime().IsZero())
		assert.Equal(t, "Debian 11.4 Released 09 July 2022", release.Description)
		assert.Equal(t, "Debian", release.Label)
		assert.Equal(t, "Debian", release.Origin)
		assert.Equal(t, "stable", release.Suite)
		assert.Equal(t, "11.4", release.Version)
		assert.NotEmpty(t, release.Digests)
		for _, v := range release.Digests {
			assert.NotEmpty(t, v.Sha256Sum)
			assert.NotEmpty(t, v.Md5Sum)
		}
		t.Fail()
	})
}
