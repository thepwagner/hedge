package oci

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Client struct {
	baseRepo string
	rt       http.RoundTripper
}

func NewClient(baseRepo string) *Client {
	return &Client{baseRepo: baseRepo, rt: http.DefaultTransport}
}

func (c *Client) GetTags(ctx context.Context, reference string) ([]string, error) {
	ref, err := name.ParseReference(path.Join(c.baseRepo, reference))
	if err != nil {
		return nil, fmt.Errorf("parsing refernce: %w", err)
	}

	tags, err := remote.List(ref.Context(), remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithTransport(c.rt))
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	return tags, nil
}
