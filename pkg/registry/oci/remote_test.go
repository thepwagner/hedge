package oci_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/registry/oci"
)

func TestRemote(t *testing.T) {
	r := oci.NewClient("registry-1.docker.io")

	ctx := context.Background()
	tags, err := r.GetTags(ctx, "library/alpine")
	require.NoError(t, err)
	t.Log("tags", tags)
	assert.NotEmpty(t, tags)
	assert.Contains(t, tags, "3.16.2")
}
