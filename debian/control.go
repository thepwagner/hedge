package debian

import (
	"fmt"
	"io"
)

type kv struct {
	k, v string
}

func writeKV(w io.Writer, entries ...kv) error {
	for _, e := range entries {
		if e.v == "" {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s: %s\n", e.k, e.v); err != nil {
			return fmt.Errorf("writing %s: %w", e.k, err)
		}
	}
	return nil
}
