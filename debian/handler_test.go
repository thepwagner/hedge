package debian_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/debian"
)

func TestDebianHandler(t *testing.T) {
	kr, err := openpgp.ReadArmoredKeyRing(strings.NewReader(signingKey))
	require.NoError(t, err)

	h := debian.NewHandler(logr.Discard(), func(context.Context, string) (*debian.Release, error) {
		return &debian.Release{
			Codename: "bullseye",
		}, nil
	}, kr)

	req, _ := http.NewRequest("GET", "/debian/dists/bullseye/InRelease", nil)
	resp := httptest.NewRecorder()
	h.HandleInRelease(resp, req)
	t.Log(resp.Body.String())

	block, rest := clearsign.Decode(resp.Body.Bytes())
	require.NotNil(t, block)
	assert.Empty(t, rest)

	_, err = openpgp.CheckDetachedSignature(kr, bytes.NewReader(block.Bytes), block.ArmoredSignature.Body, nil)
	require.NoError(t, err)

	t.Log(resp.Body.String())
}

const signingKey = `-----BEGIN PGP PRIVATE KEY BLOCK-----

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
X7FRWX8dEGBhh2Q=
=kN4H
-----END PGP PRIVATE KEY BLOCK-----`
