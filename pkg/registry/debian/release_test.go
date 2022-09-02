package debian_test

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/registry/debian"
)

func TestParseRelease(t *testing.T) {
	keyData, err := os.ReadFile("testdata/bullseye_pubkey.txt")
	require.NoError(t, err)
	key, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(keyData))
	require.NoError(t, err)
	b, err := os.ReadFile("testdata/bullseye_InRelease")
	require.NoError(t, err)

	rg, err := debian.ParseReleaseFile(b, key)
	require.NoError(t, err)
	r, err := debian.ReleaseFromParagraph(rg)
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

// func TestWriteReleaseFile(t *testing.T) {
// 	rel := debian.Release{
// 		Origin:   "Debian",
// 		Codename: "bullseye",
// 	}
// 	pkgs := map[debian.Architecture][]debian.Package{
// 		"amd64": {
// 			{Package: "foo", Version: "1.0"},
// 		},
// 	}

// 	var buf bytes.Buffer
// 	err := debian.WriteReleaseFile(context.Background(), rel, pkgs, &buf)
// 	require.NoError(t, err)

// 	assert.Equal(t, `Codename: bullseye
// Origin: Debian
// MD5Sum:
//   496dc54a775e228596fea4d6510532c7 26 main/binary-amd64/Packages
//   d84f4fd0a94f0f586cb63691b38c4589 50 main/binary-amd64/Packages.gz
// SHA256:
//   58d78f21ebcded90d2e20cf81eb98622b95f19e577893a2acc109df9b5e6128b 26 main/binary-amd64/Packages
//   fe01b3ec9920bbd6fc4b8ed887711edd07a7b0a06605f9cca91f2f147f6ee1eb 50 main/binary-amd64/Packages.gz
// `, buf.String())
// }
