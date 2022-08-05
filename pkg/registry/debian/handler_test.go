package debian_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/pkg/registry/debian"
)

func TestDebianHandler(t *testing.T) {
	// Mock bullseye mirror using test artifacts
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var fn string
		switch r.URL.Path {
		case "/dists/bullseye/InRelease":
			fn = "testdata/bullseye_InRelease"
		case "/dists/bullseye/contrib/binary-amd64/Packages.gz":
			fn = "testdata/bullseye_Packages.gz"
		default:
			assert.Fail(t, "unexpected path", r.URL.Path)
		}
		f, err := os.Open(fn)
		require.NoError(t, err)
		defer f.Close()
		_, err = io.Copy(w, f)
		require.NoError(t, err)
	}))
	defer srv.Close()

	// Handler that sources mock mirror:
	pubKey, err := ioutil.ReadFile("testdata/bullseye_pubkey.txt")
	require.NoError(t, err)
	h, err := debian.NewHandler(observability.NoopTracer, &http.Client{}, "testdata/config", map[string]*debian.RepositoryConfig{
		"bullseye": {
			KeyPath: "testdata/privkey.txt",
			Source: debian.SourceConfig{
				Upstream: &debian.UpstreamConfig{
					URL:           srv.URL,
					Release:       "bullseye",
					Architectures: []string{"amd64"},
					Components:    []string{"contrib"},
					Key:           string(pubKey),
				},
			},
		},
	})
	require.NoError(t, err)

	// Request the signed release file:
	req, _ := http.NewRequest("GET", "/debian/dists/bullseye/InRelease", nil)
	req = mux.SetURLVars(req, map[string]string{"dist": "bullseye"})
	resp := httptest.NewRecorder()
	h.HandleInRelease(resp, req)
	t.Log(resp.Body.String())

	// Verify signature:
	block, rest := clearsign.Decode(resp.Body.Bytes())
	require.NotNil(t, block)
	assert.Empty(t, rest)
	kr, err := debian.ReadArmoredKeyRingFile("testdata/privkey.txt")
	require.NoError(t, err)
	_, err = openpgp.CheckDetachedSignature(kr, bytes.NewReader(block.Bytes), block.ArmoredSignature.Body, nil)
	require.NoError(t, err)
}
