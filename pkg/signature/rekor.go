package signature

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/sigstore/cosign/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/pkg/cosign"
	rekor "github.com/sigstore/rekor/pkg/client"
	"github.com/sigstore/rekor/pkg/generated/client"
	"github.com/sigstore/rekor/pkg/generated/client/entries"
	"github.com/sigstore/rekor/pkg/generated/client/index"
	"github.com/sigstore/rekor/pkg/generated/models"
	rekortypes "github.com/sigstore/rekor/pkg/types"
	hashedrekord_v001 "github.com/sigstore/rekor/pkg/types/hashedrekord/v0.0.1"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

type RekorFinder struct {
	rekor *client.Rekor
}

type ActionsWorkflowID struct {
	Trigger      string `json:"trigger"`
	Sha          string `json:"sha"`
	WorkflowName string `json:"workflowName"`
	Repository   string `json:"repository"`
	Ref          string `json:"ref"`
}

type RekorEntry struct {
	Issuer string `json:"issuer"`
	Email  string `json:"email"`

	GitHubActions ActionsWorkflowID `json:"githubActions"`
}

func NewRekorFinder(httpClient *http.Client) (*RekorFinder, error) {
	client, err := rekor.GetRekorClient("https://rekor.sigstore.dev/")
	if err != nil {
		return nil, err
	}
	client.Transport.(*httptransport.Runtime).Transport = httpClient.Transport

	return &RekorFinder{
		rekor: client,
	}, nil
}

func (r *RekorFinder) GetSignature(ctx context.Context, digest []byte) (*RekorEntry, error) {
	res, err := r.rekor.Index.SearchIndex(&index.SearchIndexParams{
		Context: ctx,
		Query: &models.SearchIndex{
			Hash: fmt.Sprintf("sha256:%x", digest),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("searching index: %w", err)
	}
	if len(res.Payload) == 0 {
		return nil, nil
	}

	entryUUID := res.Payload[0]
	entryRes, err := r.rekor.Entries.GetLogEntryByUUID(&entries.GetLogEntryByUUIDParams{
		Context:   ctx,
		EntryUUID: entryUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("searching index: %w", err)
	}
	for _, payload := range entryRes.GetPayload() {
		dec, err := base64.StdEncoding.DecodeString(payload.Body.(string))
		if err != nil {
			return nil, fmt.Errorf("decoding entry: %w", err)
		}
		pe, err := models.UnmarshalProposedEntry(bytes.NewReader(dec), runtime.JSONConsumer())
		if err != nil {
			return nil, fmt.Errorf("unmarshaling proposed entry: %w", err)
		}

		entry, err := rekortypes.NewEntry(pe)
		if err != nil {
			return nil, fmt.Errorf("parsing entry: %w", err)
		}
		he := entry.(*hashedrekord_v001.V001Entry)
		sig := he.HashedRekordObj.Signature

		block, _ := pem.Decode(sig.PublicKey.Content)
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing public key: %w", err)
		}

		roots, _ := fulcio.GetRoots()
		inters, _ := fulcio.GetIntermediates()
		verifier, err := cosign.ValidateAndUnpackCert(cert, &cosign.CheckOpts{
			RootCerts:         roots,
			IntermediateCerts: inters,
		})
		if err != nil {
			return nil, fmt.Errorf("loading public key: %w", err)
		}

		digest, _ := hex.DecodeString(*he.HashedRekordObj.Data.Hash.Value)
		err = verifier.VerifySignature(bytes.NewReader(he.HashedRekordObj.Signature.Content), nil, options.WithDigest(digest))
		if err != nil {
			return nil, fmt.Errorf("verifying signature: %w", err)
		}

		ce := cosign.CertExtensions{Cert: cert}
		rek := &RekorEntry{
			Issuer: ce.GetIssuer(),
			GitHubActions: ActionsWorkflowID{
				Trigger:      ce.GetCertExtensionGithubWorkflowTrigger(),
				Sha:          ce.GetExtensionGithubWorkflowSha(),
				WorkflowName: ce.GetCertExtensionGithubWorkflowName(),
				Repository:   ce.GetCertExtensionGithubWorkflowRepository(),
				Ref:          ce.GetCertExtensionGithubWorkflowRef(),
			},
		}
		if len(cert.EmailAddresses) > 0 {
			rek.Email = cert.EmailAddresses[0]
		}
		return rek, nil
	}
	return nil, nil
}
