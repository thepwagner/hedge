package debian_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/registry/debian"
)

func TestCompression(t *testing.T) {
	data := []byte("hello")
	cases := map[debian.Compression]int{
		debian.CompressionNone: 5,
		debian.CompressionGZIP: 29,
		debian.CompressionXZ:   64,
	}

	for compression, expectedSize := range cases {
		t.Run(string(compression), func(t *testing.T) {
			var buf bytes.Buffer
			err := compression.Compress(&buf, bytes.NewReader(data))
			require.NoError(t, err)
			assert.Equal(t, expectedSize, buf.Len())

			compressed := buf.Bytes()
			buf.Reset()
			err = compression.Decompress(&buf, bytes.NewReader(compressed))
			require.NoError(t, err)
			assert.Equal(t, data, buf.Bytes())
		})
	}
}
