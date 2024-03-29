package debian_test

import (
	"compress/gzip"
	"context"
	"encoding/hex"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/pkg/registry/debian"
)

func TestParsePackages(t *testing.T) {
	// Parse a copy of "contrib" Packages (the smallest component)
	f, err := os.Open("testdata/bullseye_Packages.gz")
	require.NoError(t, err)
	defer f.Close()
	gzR, err := gzip.NewReader(f)
	require.NoError(t, err)

	pp := debian.NewParser(observability.NoopTracer)
	pkgs, err := pp.Packages(context.Background(), gzR)
	require.NoError(t, err)
	assert.Len(t, pkgs, 297)

	pkg := pkgs[0]
	assert.Equal(t, "alien-arena", pkg.Name)
	assert.Equal(t, "7.66+dfsg-6", pkg.Version)
	assert.Equal(t, uint64(2017), pkg.InstalledSize)
	assert.Equal(t, "Debian Games Team <pkg-games-devel@lists.alioth.debian.org>", pkg.Maintainer)
	assert.Equal(t, "amd64", pkg.Architecture)
	assert.Equal(t, []string{
		"libc6 (>= 2.17)",
		"libcurl3-gnutls (>= 7.16.2)",
		"libfreetype6 (>= 2.3.5)",
		"libgcc-s1 (>= 3.0)",
		"libjpeg62-turbo (>= 1.3.1)",
		"libstdc++6 (>= 5)",
		"libvorbisfile3 (>= 1.1.2)",
		"libx11-6",
		"libxxf86vm1",
		"zlib1g (>= 1:1.1.4)",
		"libopenal1",
		"alien-arena-data",
	}, pkg.Depends)
	assert.Equal(t, "Standalone 3D first person online deathmatch shooter", pkg.Description)
	assert.Equal(t, "http://red.planetarena.org", pkg.Homepage)
	assert.Equal(t, []string{
		"game::fps",
		"hardware::input:keyboard",
		"hardware::input:mouse",
		"hardware::opengl",
		"implemented-in::c",
		"interface::3d",
		"interface::graphical",
		"interface::x11",
		"network::client",
		"role::program",
		"uitoolkit::sdl",
		"use::gameplaying",
		"x11::application",
	}, pkg.Tags)
	assert.Equal(t, "contrib/games", pkg.Section)
	assert.Equal(t, "optional", pkg.Priority)
	assert.Equal(t, "pool/contrib/a/alien-arena/alien-arena_7.66+dfsg-6_amd64.deb", pkg.Filename)
	assert.Equal(t, uint64(776388), pkg.Size)
	digest, _ := hex.DecodeString("3fcd4894851b100a4da3f05b94e13fd64e639b309fba4dda979052a422c31e8e")
	assert.Equal(t, digest, pkg.Sha256)
}
