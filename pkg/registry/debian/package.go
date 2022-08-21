package debian

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/thepwagner/hedge/pkg/signature"
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

func PackageFromParagraph(graph Paragraph) (Package, error) {
	var pkg Package
	if err := mapstructure.Decode(graph, &pkg); err != nil {
		return pkg, fmt.Errorf("parsing package: %w", err)
	}
	return pkg, nil
}
