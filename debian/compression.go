package debian

import (
	"compress/gzip"
	"fmt"
	"io"

	"github.com/ulikunitz/xz"
)

type Compression string

var (
	CompressionNone = Compression("")
	CompressionGZIP = Compression("gzip")
	CompressionXZ   = Compression("xz")
)

func FromExtension(extension string) Compression {
	switch extension {
	case ".gz":
		return CompressionGZIP
	case ".xz":
		return CompressionXZ
	default:
		return CompressionNone
	}
}

func (c Compression) Compress(dst io.Writer, src io.Reader) error {
	switch c {
	case CompressionNone:

	case CompressionGZIP:
		w := gzip.NewWriter(dst)
		defer w.Close()
		dst = w

	case CompressionXZ:
		w, err := xz.NewWriter(dst)
		if err != nil {
			return fmt.Errorf("creating xz writer: %w", err)
		}
		defer w.Close()
		dst = w

	default:
		return fmt.Errorf("unknown compression: %s", c)
	}

	_, err := io.Copy(dst, src)
	return err
}

func (c Compression) Decompress(dst io.Writer, src io.Reader) error {
	switch c {
	case CompressionNone:

	case CompressionGZIP:
		r, err := gzip.NewReader(src)
		if err != nil {
			return fmt.Errorf("creating gzip reader: %w", err)
		}
		defer r.Close()
		src = r

	case CompressionXZ:
		r, err := xz.NewReader(src)
		if err != nil {
			return fmt.Errorf("creating gzip reader: %w", err)
		}
		src = r

	default:
		return fmt.Errorf("unknown compression: %s", c)
	}

	_, err := io.Copy(dst, src)
	return err
}
