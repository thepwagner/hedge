package debian_test

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/pkg/registry/debian"
)

func TestPackageParser_PackageFromDeb(t *testing.T) {
	f, err := os.Open("testdata/testpkg_1.2.3_amd64.deb")
	require.NoError(t, err)
	defer f.Close()

	parser := debian.NewParser(observability.NoopTracer)
	pkg, err := parser.PackageFromDeb(context.Background(), f)
	require.NoError(t, err)
	assert.Equal(t, "testpkg", pkg.Name)
	assert.Equal(t, "1.2.3", pkg.Version)
	assert.Equal(t, "pwagner", pkg.Maintainer)
	assert.Equal(t, "hansel virtual package", pkg.Description)
	assert.Equal(t, "optional", pkg.Priority)
	assert.Equal(t, "amd64", pkg.Architecture)
}

func TestParser_Release(t *testing.T) {
	keyData, err := os.ReadFile("testdata/bullseye_pubkey.txt")
	require.NoError(t, err)
	key, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(keyData))
	require.NoError(t, err)
	f, err := os.Open("testdata/bullseye_InRelease")
	require.NoError(t, err)
	defer f.Close()

	parser := debian.NewParser(observability.NoopTracer)
	r, err := parser.Release(context.Background(), f, key)
	require.NoError(t, err)

	assert.Equal(t, "Debian", r.Origin)
	assert.Equal(t, "Debian", r.Label)
	assert.Equal(t, "stable", r.Suite)
	assert.Equal(t, "11.4", r.Version)
	assert.Equal(t, "bullseye", r.Codename)
	assert.Equal(t, "https://metadata.ftp-master.debian.org/changelogs/@CHANGEPATH@_changelog", r.Changelogs)
	date, _ := time.Parse(time.RFC1123, "Sat, 09 Jul 2022 09:43:23 UTC")
	assert.Equal(t, date, r.Date.AsTime())
	assert.Equal(t, []string{
		"all",
		"amd64",
		"arm64",
		"armel",
		"armhf",
		"i386",
		"mips64el",
		"mipsel",
		"ppc64el",
		"s390x",
	}, r.Architectures)
	assert.Equal(t, []string{
		"main",
		"contrib",
		"non-free",
	}, r.Components)
	assert.Equal(t, "Debian 11.4 Released 09 July 2022", r.Description)
}
