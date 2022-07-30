package debian

import (
	"fmt"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
)

func ReadArmoredKeyRingFile(fn string) (openpgp.EntityList, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, fmt.Errorf("opening keyring: %w", err)
	}
	defer f.Close()

	kr, err := openpgp.ReadArmoredKeyRing(f)
	if err != nil {
		return nil, fmt.Errorf("reading keyring: %w", err)
	}
	return kr, nil
}
