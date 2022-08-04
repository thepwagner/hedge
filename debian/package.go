package debian

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
)

type Package struct {
	Package          string
	Source           string
	Version          string
	InstalledSizeRaw string `mapstructure:"Installed-Size"`
	Maintainer       string
	DependsRaw       string `mapstructure:"Depends"`
	PreDepends       string
	Section          string
	TagRaw           string `mapstructure:"Tag"`
	Description      string
	Homepage         string
	Priority         string
	Architecture     string
	Filename         string
	SizeRaw          string `mapstructure:"Size"`
	Sha256           string
}

func (p Package) GetName() string     { return p.Package }
func (p Package) GetPriority() string { return p.Priority }

func (p Package) Depends() []string {
	return strings.Split(p.DependsRaw, ", ")
}

func (p Package) Tags() []string {
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

func WritePackages(out io.Writer, packages ...Package) error {
	graphs := make([]Paragraph, 0, len(packages))
	for _, pkg := range packages {
		graph, err := pkg.Paragraph()
		if err != nil {
			return err
		}
		graphs = append(graphs, graph)
	}
	return WriteControlFile(out, graphs...)
}
