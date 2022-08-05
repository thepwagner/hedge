package signature_test

import (
	"context"
	"encoding/hex"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/signature"
)

func TestRekor_Cosign(t *testing.T) {
	client, err := signature.NewRekorFinder(http.DefaultClient)
	require.NoError(t, err)

	digest, _ := hex.DecodeString("18144be9b06478496f08937a013a3fb7ae90e51a1da8bf5682d6b13f88649860")

	signer, err := client.GetSignature(context.Background(), digest)
	require.NoError(t, err)

	assert.Equal(t, "https://accounts.google.com", signer.Issuer)
	assert.Equal(t, "keyless@projectsigstore.iam.gserviceaccount.com", signer.Email)
}

func TestRekor_GitSign(t *testing.T) {
	client, err := signature.NewRekorFinder(http.DefaultClient)
	require.NoError(t, err)

	digest, _ := hex.DecodeString("3b459e717915526a0eb5eb0aaece60a6aed55ffb37f568a19f5ddff07015cda4")

	signer, err := client.GetSignature(context.Background(), digest)
	require.NoError(t, err)

	assert.Equal(t, "https://token.actions.githubusercontent.com", signer.Issuer)
	assert.Equal(t, "", signer.Email)
	assert.Equal(t, "push", signer.GitHubActions.Trigger)
	assert.Equal(t, "4bc492cc12d32998473bddd01c31f2a97b94fd1c", signer.GitHubActions.Sha)
	assert.Equal(t, "release", signer.GitHubActions.WorkflowName)
	assert.Equal(t, "sigstore/gitsign", signer.GitHubActions.Repository)
	assert.Equal(t, "refs/tags/v0.2.0", signer.GitHubActions.Ref)
}
