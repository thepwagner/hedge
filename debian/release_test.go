package debian_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/debian"
)

func TestParseRelease(t *testing.T) {
	kr := loadKey(t, "testdata/bullseye_pubkey.txt")
	b, err := ioutil.ReadFile("testdata/bullseye_InRelease")
	require.NoError(t, err)

	r, err := debian.ParseReleaseFile(kr, b)
	require.NoError(t, err)

	assert.Equal(t, "Debian", r.Origin)
	assert.Equal(t, "Debian", r.Label)
	assert.Equal(t, "stable", r.Suite)
	assert.Equal(t, "11.4", r.Version)
	assert.Equal(t, "bullseye", r.Codename)
	t.Fail()
}

func loadKey(tb testing.TB, keyfile string) openpgp.EntityList {
	tb.Helper()
	f, err := os.Open(keyfile)
	require.NoError(tb, err)
	defer f.Close()

	keyring, err := openpgp.ReadArmoredKeyRing(f)
	require.NoError(tb, err)
	return keyring
}
