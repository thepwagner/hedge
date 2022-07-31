package debian_test

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/debian"
)

func TestParseRelease(t *testing.T) {
	kr := loadKey(t, "testdata/bullseye_pubkey.txt")
	b, err := ioutil.ReadFile("testdata/bullseye_InRelease")
	require.NoError(t, err)

	rg, err := debian.ParseReleaseFile(b, kr)
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
	assert.Equal(t, date, r.Date())
	assert.Equal(t, []debian.Architecture{
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
	}, r.Architectures())
	assert.Equal(t, []debian.Component{
		"main",
		"contrib",
		"non-free",
	}, r.Components())
	assert.Equal(t, "Debian 11.4 Released 09 July 2022", r.Description)
}

func TestWriteReleaseFile(t *testing.T) {
	r := debian.Release{}

	var buf bytes.Buffer
	err := debian.WriteReleaseFile(r, nil, &buf)
	require.NoError(t, err)
}

func loadKey(tb testing.TB, keyfile string) openpgp.EntityList {
	tb.Helper()
	keyring, err := debian.ReadArmoredKeyRingFile(keyfile)
	require.NoError(tb, err)
	return keyring
}
