package debian

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
)

type Release struct {
	Components    []string
	Architectures []string

	Date time.Time

	// Optional
	Description string
	Origin      string
	Label       string
	Version     string
	Codename    string `yaml:"codename"`
	Suite       string
	Packages    map[string]string `yaml:"packages"`
}

func ParseReleaseFile(kr openpgp.EntityList, data []byte) (*Release, error) {
	block, _ := clearsign.Decode(data)
	_, err := openpgp.CheckDetachedSignature(kr, bytes.NewReader(block.Bytes), block.ArmoredSignature.Body, nil)
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	var r Release
	for _, line := range strings.Split(string(block.Plaintext), "\n") {
		split := strings.SplitN(line, ":", 2)
		if len(split) == 2 {
			switch key := split[0]; key {
			case "Origin":
				r.Origin = strings.TrimLeft(split[1], " ")
			}
		}
	}

	return &r, nil
}

type kv struct {
	k, v string
}

func WriteReleaseFile(r Release, w io.Writer) error {
	now := time.Now()

	pkgDigests, err := PackageHashes("amd64", hackPackages...)
	if err != nil {
		return fmt.Errorf("calculating package hashes: %w", err)
	}
	digests := make([]string, 0, len(pkgDigests))
	for _, d := range pkgDigests {
		digests = append(digests, fmt.Sprintf(" %x %d %s", d.Digest, d.Size, d.Path))
	}

	m := []kv{
		{k: "Origin", v: r.Origin},
		{k: "Label", v: r.Label},
		{k: "Suite", v: r.Suite},
		{k: "Version", v: r.Version},
		{k: "Codename", v: r.Codename},
		{k: "Changelogs", v: ""},
		{k: "Date", v: now.UTC().Format(time.RFC1123)},
		{k: "No-Support-for-Architecture-all", v: "Packages"},
		{k: "Architectures", v: strings.Join(r.Architectures, " ")},
		{k: "Components", v: strings.Join(r.Components, " ")},
		{k: "Description", v: r.Description},
		{k: "SHA256", v: "\n" + strings.Join(digests, "\n")},
	}

	if err := writeKV(w, m...); err != nil {
		return err
	}
	return nil
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
