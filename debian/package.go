package debian

import (
	"fmt"
	"io"
)

type Package struct {
	Package       string
	Source        string
	Version       string
	InstalledSize int
	Maintainer    string
	Depends       []string
	PreDepends    []string
	Section       string
	Description   string
	Priority      string
	Architecture  string
	Filename      string
	Size          int
	Sha256        string
}

var hackPackages = []Package{
	{
		Package:      "meow",
		Version:      "5.1-2+deb11u1",
		Architecture: "amd64",
		Filename:     "pool/main/m/meow/meow_5.1-2+deb11u1_amd64.deb",
		Size:         5588508,
		Sha256:       "610e9f9c41be18af516dd64a6dc1316dbfe1bb8989c52bafa556de9e381d3e29",
		Description:  "meow",
	},
	{
		Package:      "woof",
		Version:      "5.1-2+deb11u1",
		Architecture: "amd64",
		Filename:     "pool/main/m/meow/meow_5.1-2+deb11u1_amd64.deb",
		Size:         5588508,
		Sha256:       "610e9f9c41be18af516dd64a6dc1316dbfe1bb8989c52bafa556de9e381d3e29",
		Description:  "woof",
	},
}

func WritePackages(out io.Writer, packages ...Package) error {
	for _, pkg := range packages {
		if err := writeKV(out,
			kv{k: "Package", v: pkg.Package},
			kv{k: "Version", v: pkg.Version},
			kv{k: "Architecture", v: pkg.Architecture},
			kv{k: "Description", v: pkg.Description},
			kv{k: "Filename", v: pkg.Filename},
			kv{k: "Size", v: fmt.Sprintf("%d", pkg.Size)},
			kv{k: "SHA256", v: pkg.Sha256},
		); err != nil {
			return err
		}
		fmt.Fprintln(out)
	}

	fmt.Fprintln(out)
	return nil
}
