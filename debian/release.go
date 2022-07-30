package debian

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strconv"
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
	Changelogs  string
	Packages    map[string]string `yaml:"packages"`
}

var digestLineRE = regexp.MustCompile(`^ ([0-9a-f]+)\s+([0-9]+)\s+([^/]+)/binary-([^/]+)/Packages\.xz$`)

type ParseReleaseOptions struct {
	SigningKey    openpgp.EntityList
	Components    []string
	Architectures []string
}

func ParseReleaseFile(data []byte, opts ParseReleaseOptions) (*Release, error) {
	// Verify signature:
	block, _ := clearsign.Decode(data)
	_, err := openpgp.CheckDetachedSignature(opts.SigningKey, bytes.NewReader(block.Bytes), block.ArmoredSignature.Body, nil)
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	comps := make(map[string]struct{}, len(opts.Components))
	for _, c := range opts.Components {
		comps[c] = struct{}{}
	}
	archs := make(map[string]struct{}, len(opts.Architectures))
	for _, a := range opts.Architectures {
		archs[a] = struct{}{}
	}

	// Parse file:
	var r Release
	var parsingDigests bool
	var packages []PackagesDigest
	for _, line := range strings.Split(string(block.Plaintext), "\n") {
		split := strings.SplitN(line, ":", 2)
		if len(split) != 2 {
			if !parsingDigests {
				continue
			}
			if len(line) == 0 || line[0] != ' ' && line[0] != '\t' {
				continue
			}

			digestMatch := digestLineRE.FindStringSubmatch(line)
			if len(digestMatch) == 0 {
				continue
			}

			digest, err := hex.DecodeString(digestMatch[1])
			if err != nil {
				return nil, fmt.Errorf("decoding digest: %w", err)
			}

			size, err := strconv.Atoi(digestMatch[2])
			if err != nil {
				return nil, fmt.Errorf("decoding size: %w", err)
			}
			component := digestMatch[3]
			if _, ok := comps[component]; len(comps) > 0 && !ok {
				continue
			}
			arch := digestMatch[4]
			if _, ok := archs[arch]; len(archs) > 0 && !ok {
				continue
			}

			packages = append(packages, PackagesDigest{
				Digest: digest,
				Size:   size,
				Path:   fmt.Sprintf("%s/binary-%s/Packages.xz", component, arch),
			})
			continue
		}

		parsingDigests = false

		switch key := split[0]; key {
		case "Origin":
			r.Origin = strings.TrimLeft(split[1], " ")
		case "Label":
			r.Label = strings.TrimLeft(split[1], " ")
		case "Suite":
			r.Suite = strings.TrimLeft(split[1], " ")
		case "Version":
			r.Version = strings.TrimLeft(split[1], " ")
		case "Codename":
			r.Codename = strings.TrimLeft(split[1], " ")
		case "Changelogs":
			r.Changelogs = strings.TrimLeft(split[1], " ")
		case "Date":
			date, err := time.Parse(time.RFC1123, strings.TrimLeft(split[1], " "))
			if err != nil {
				return nil, fmt.Errorf("parsing date: %w", err)
			}
			r.Date = date
		case "Architectures":
			r.Architectures = strings.Split(strings.TrimLeft(split[1], " "), " ")
		case "Components":
			r.Components = strings.Split(strings.TrimLeft(split[1], " "), " ")
		case "Description":
			r.Description = strings.TrimLeft(split[1], " ")
		case "MD5Sum", "Acquire-By-Hash", "No-Support-for-Architecture-all":
			// Ignore
		case "SHA256":
			parsingDigests = true
		default:
			return nil, fmt.Errorf("unknown key: %s", key)
		}
	}

	var sum int
	for _, d := range packages {
		fmt.Printf(" %x %d %s\n", d.Digest, d.Size, d.Path)
		sum += d.Size
	}
	fmt.Println("fetching packages", "total_bytes", sum)

	return &r, nil
}

func WriteReleaseFile(r Release, w io.Writer) error {
	pkgDigests, err := PackageHashes("amd64", hackPackages...)
	if err != nil {
		return fmt.Errorf("calculating package hashes: %w", err)
	}
	digests := make([]string, 0, len(pkgDigests))
	for _, d := range pkgDigests {
		digests = append(digests, fmt.Sprintf(" %x %d %s", d.Digest, d.Size, d.Path))
	}

	var date time.Time
	if r.Date.IsZero() {
		date = time.Now()
	} else {
		date = r.Date
	}

	m := []kv{
		{k: "Origin", v: r.Origin},
		{k: "Label", v: r.Label},
		{k: "Suite", v: r.Suite},
		{k: "Version", v: r.Version},
		{k: "Codename", v: r.Codename},
		{k: "Changelogs", v: r.Changelogs},
		{k: "Date", v: date.UTC().Format(time.RFC1123)},
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

type PackagesDigest struct {
	Path   string
	Size   int
	Digest []byte
}

func PackageHashes(arch string, packages ...Package) ([]PackagesDigest, error) {
	var buf bytes.Buffer
	if err := WritePackages(&buf, packages...); err != nil {
		return nil, err
	}
	b := buf.Bytes()

	var digests []PackagesDigest
	for _, compression := range []Compression{CompressionNone, CompressionXZ} {
		var buf bytes.Buffer
		if err := compression.Compress(&buf, bytes.NewReader(b)); err != nil {
			return nil, err
		}
		size := buf.Len()
		sha := sha256.Sum256(buf.Bytes())
		digests = append(digests, PackagesDigest{
			Path:   fmt.Sprintf("main/binary-%s/Packages%s", arch, compression.Extension()),
			Size:   size,
			Digest: sha[:],
		})
	}

	return digests, nil
}
