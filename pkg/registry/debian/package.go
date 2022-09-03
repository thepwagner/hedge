package debian

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/thepwagner/hedge/proto/hedge/v1"
)

func PackageFromParagraph(graph Paragraph) (*hedge.DebianPackage, error) {
	pkg := hedge.DebianPackage{
		Name:          graph["Package"],
		Architecture:  graph["Architecture"],
		Breaks:        strings.Split(graph["Breaks"], ", "),
		Conflicts:     strings.Split(graph["Conflicts"], ", "),
		Depends:       strings.Split(graph["Depends"], ", "),
		Description:   graph["Description"],
		Enhances:      strings.Split(graph["Enhances"], ", "),
		Essential:     graph["Essential"] == "yes",
		Filename:      graph["Filename"],
		Homepage:      graph["Homepage"],
		Important:     graph["Important"] == "yes",
		LuaVersions:   strings.Split(graph["LuaVersions"], " "),
		Maintainer:    graph["Maintainer"],
		Multiarch:     graph["Multi-Arch"],
		PreDepends:    strings.Split(graph["Pre-Depends"], ", "),
		Priority:      graph["Priority"],
		Protected:     graph["Protected"] == "yes",
		Provides:      strings.Split(graph["Provides"], ", "),
		PythonVersion: graph["Python-Version"],
		Recommends:    strings.Split(graph["Recommends"], ", "),
		Replaces:      strings.Split(graph["Replaces"], ", "),
		RubyVersions:  strings.Split(graph["RubyVersions"], " "),
		Section:       graph["Section"],
		Source:        graph["Source"],
		Suggests:      strings.Split(graph["Suggests"], ", "),
		Tags:          strings.Split(graph["Tag"], ", "),
		Version:       graph["Version"],
	}

	if v, ok := graph["Installed-Size"]; ok {
		i, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid Installed-Size: %s", v)
		}
		pkg.InstalledSize = uint64(i)
	}
	if v, ok := graph["Size"]; ok {
		i, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid Size: %s", v)
		}
		pkg.Size = uint64(i)
	}

	if v, ok := graph["MD5Sum"]; ok {
		digest, err := hex.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("invalid MD5sum: %s", v)
		}
		pkg.Md5Sum = digest
	}
	if v, ok := graph["SHA256"]; ok {
		digest, err := hex.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("invalid SHA256: %s", v)
		}
		pkg.Sha256 = digest
	}

	for k, v := range graph {
		switch k {
		case "Package", "Architecture", "Breaks", "Conflicts", "Depends", "Description", "Enhances", "Essential", "Filename", "Homepage", "Important", "Installed-Size", "Lua-Versions", "Maintainer", "MD5sum", "Multi-Arch",
			"Pre-Depends", "Priority", "Protected", "Provides", "Python-Version", "Recommends", "Replaces", "Ruby-Versions", "Section", "SHA256", "Size", "Source", "Suggests", "Tag", "Version":
			// Mapped above
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

func boolToDebian(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func ParagraphFromPackage(pkg *hedge.DebianPackage) Paragraph {
	return Paragraph{
		"Package":        pkg.Name,
		"Architecture":   pkg.Architecture,
		"Breaks":         strings.Join(pkg.Breaks, ", "),
		"Conflicts":      strings.Join(pkg.Conflicts, ", "),
		"Depends":        strings.Join(pkg.Depends, ", "),
		"Description":    pkg.Description,
		"Enhances":       strings.Join(pkg.Enhances, ", "),
		"Essential":      boolToDebian(pkg.Essential),
		"Filename":       pkg.Filename,
		"Homepage":       pkg.Homepage,
		"Important":      boolToDebian(pkg.Important),
		"Installed-Size": strconv.FormatUint(pkg.InstalledSize, 10),
		"LuaVersions":    strings.Join(pkg.LuaVersions, " "),
		"Maintainer":     pkg.Maintainer,
		"MD5Sum":         hex.EncodeToString(pkg.Md5Sum),
		"Multi-Arch":     pkg.Multiarch,
		"Pre-Depends":    strings.Join(pkg.PreDepends, ", "),
		"Priority":       pkg.Priority,
		"Protected":      boolToDebian(pkg.Protected),
		"Provides":       strings.Join(pkg.Provides, ", "),
		"Python-Version": pkg.PythonVersion,
		"Recommends":     strings.Join(pkg.Recommends, ", "),
		"Replaces":       strings.Join(pkg.Replaces, ", "),
		"Ruby-Versions":  strings.Join(pkg.RubyVersions, " "),
		"Section":        pkg.Section,
		"Source":         pkg.Source,
		"SHA256":         hex.EncodeToString(pkg.Sha256),
		"Size":           strconv.FormatUint(pkg.Size, 10),
		"Suggests":       strings.Join(pkg.Suggests, ", "),
		"Tag":            strings.Join(pkg.Tags, ", "),
		"Version":        pkg.Version,
	}
}
