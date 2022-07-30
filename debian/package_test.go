package debian_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/debian"
)

func TestWritePackages(t *testing.T) {
	var buf bytes.Buffer
	pkgs := []debian.Package{
		{
			Package: "bash",
			Version: "5.1-2+deb11u1",
		},
	}

	err := debian.WritePackages(&buf, pkgs...)
	require.NoError(t, err)

	t.Log(buf.String())
	assert.Equal(t, "", buf.String())
}
