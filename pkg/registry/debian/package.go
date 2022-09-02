package debian

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/thepwagner/hedge/pkg/signature"
	"github.com/thepwagner/hedge/proto/hedge/v1"
)

type Package struct {
	Package          string `json:"name,omitempty"`
	Source           string `json:"source,omitempty"`
	Version          string `json:"version,omitempty"`
	InstalledSizeRaw string `json:"-" mapstructure:"Installed-Size"`
	Maintainer       string `json:"maintainer,omitempty"`
	DependsRaw       string `json:"-" mapstructure:"Depends"`
	PreDepends       string `json:"-" mapstructure:"Pre-Depends"`
	Section          string `json:"section,omitempty"`
	TagRaw           string `json:"-" mapstructure:"Tag"`
	Description      string `json:"description,omitempty"`
	Homepage         string `json:"homepage,omitempty"`
	Priority         string `json:"priority,omitempty"`
	Architecture     string `json:"architecture,omitempty"`
	Filename         string `json:"filename,omitempty"`
	SizeRaw          string `json:"-" mapstructure:"Size"`
	MD5Sum           string `json:"md5sum,omitempty"`
	Sha256           string `json:"sha256,omitempty"`
	RekorRaw         string `json:"-" mapstructure:"-"`
}

func (p Package) GetName() string     { return p.Package }
func (p Package) GetPriority() string { return p.Priority }

func (p Package) MarshalJSON() ([]byte, error) {
	var rek *signature.RekorEntry
	if p.RekorRaw != "" {
		if err := json.Unmarshal([]byte(p.RekorRaw), rek); err != nil {
			return nil, err
		}
	}

	type Alias Package
	return json.Marshal(&struct {
		*Alias
		Depends []string              `json:"depends,omitempty"`
		Tags    []string              `json:"tags,omitempty"`
		Rekor   *signature.RekorEntry `json:"rekor,omitempty"`
	}{
		Tags:    p.Tags(),
		Depends: p.Depends(),
		Alias:   (*Alias)(&p),
		Rekor:   rek,
	})
}

func (p Package) Depends() []string {
	if p.DependsRaw == "" {
		return nil
	}
	return strings.Split(p.DependsRaw, ", ")
}

func (p Package) Tags() []string {
	if p.TagRaw == "" {
		return nil
	}
	return strings.Split(p.TagRaw, ", ")
}

func (p Package) Size() int {
	i, _ := strconv.Atoi(p.SizeRaw)
	return i
}

func (p Package) InstalledSize() int {
	i, _ := strconv.Atoi(p.InstalledSizeRaw)
	return i
}

func (p Package) Paragraph() (Paragraph, error) {
	graph := Paragraph{}
	if err := mapstructure.Decode(p, &graph); err != nil {
		return nil, err
	}
	if graph["Size"] == "0" {
		delete(graph, "Size")
	}
	return graph, nil
}

func PackageFromParagraph(graph Paragraph) (*hedge.DebianPackage, error) {
	var pkg hedge.DebianPackage
	for k, v := range graph {
		switch k {
		case "Package":
			pkg.Name = v
		case "Architecture":
			pkg.Architecture = v
		case "Breaks":
			pkg.Breaks = strings.Split(v, ", ")
		case "Conflicts":
			pkg.Conflicts = strings.Split(v, ", ")
		case "Depends":
			pkg.Depends = strings.Split(v, ", ")
		case "Description":
			pkg.Description = v
		case "Enhances":
			pkg.Enhances = strings.Split(v, ", ")
		case "Essential":
			pkg.Essential = v == "yes"
		case "Filename":
			pkg.Filename = v
		case "Homepage":
			pkg.Homepage = v
		case "Important":
			pkg.Important = v == "yes"
		case "Installed-Size":
			i, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid Installed-Size: %s", v)
			}
			pkg.InstalledSize = uint64(i)
		case "Lua-Versions":
			pkg.LuaVersions = strings.Split(v, " ")
		case "Maintainer":
			pkg.Maintainer = v
		case "MD5sum":
			digest, err := hex.DecodeString(v)
			if err != nil {
				return nil, fmt.Errorf("invalid MD5sum: %s", v)
			}
			pkg.Md5Sum = digest
		case "Multi-Arch":
			pkg.Multiarch = v
		case "Pre-Depends":
			pkg.PreDepends = strings.Split(v, ", ")
		case "Priority":
			pkg.Priority = v
		case "Protected":
			pkg.Protected = v == "yes"
		case "Provides":
			pkg.Provides = v
		case "Python-Version":
			pkg.PythonVersion = v
		case "Recommends":
			pkg.Recommends = strings.Split(v, ", ")
		case "Replaces":
			pkg.Replaces = strings.Split(v, ", ")
		case "Ruby-Versions":
			pkg.RubyVersions = strings.Split(v, ", ")
		case "Section":
			pkg.Section = v
		case "SHA256":
			digest, err := hex.DecodeString(v)
			if err != nil {
				return nil, fmt.Errorf("invalid SHA256: %s", v)
			}
			pkg.Sha256 = digest
		case "Size":
			i, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid Size: %s", v)
			}
			pkg.Size = uint64(i)
		case "Source":
			pkg.Source = v
		case "Suggests":
			pkg.Suggests = strings.Split(v, ", ")
		case "Tag":
			pkg.Tags = strings.Split(v, ", ")
		case "Version":
			pkg.Version = v

		case "Build-Ids", "Built-Using", "Build-Essential",
			"Cnf-Extra-Commands", "Cnf-Ignore-Commands", "Cnf-Priority-Bonus", "Cnf-Visible-Pkgname",
			"Description-md5",
			"Efi-Vendor",
			"Gstreamer-Decoders", "Gstreamer-Elements", "Gstreamer-Encoders", "Gstreamer-Uri-Sinks", "Gstreamer-Uri-Sources", "Gstreamer-Version",
			"Ghc-Package",
			"Go-Import-Path",
			"Postgresql-Catversion",
			"Python-Egg-Name",
			"X-Cargo-Built-Using":
			// drop
		default:
			return nil, fmt.Errorf("unexpected key %q in paragraph: %v", k, v)
		}
	}
	return &pkg, nil
}
