package npm

import (
	"encoding/json"
	"fmt"
	"io"
)

func ParsePackage(in io.Reader) (*Package, error) {
	var p Package
	if err := json.NewDecoder(in).Decode(&p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, fmt.Errorf("invalid package")
	}
	return &p, nil
}

type Package struct {
	// The package name
	ID string `json:"_id"`
	// The last revision ID
	Rev string `json:"_rev"`
	// The package name
	Name string `json:"name"`
	// Description from the package.json
	Description string `json:"description"`
	// An object with at least one key, 'latest', representing dist-tags
	DistTags map[string]string `json:"dist-tags"`
	// List of all the Version objects forthe Package
	Versions map[string]Version `json:"versions"`
	// Full text of the 'latest' verion's README
	Readme      string       `json:"readme"`
	Maintainers []RemoteUser `json:"maintainers"`
	// Object creating a 'created' and 'modified' time stamp
	Times map[string]string `json:"time"`
	// Object with 'name', 'email' and 'url' of the author listed in package.json
	Author RemoteUser `json:"author"`
	// Object with 'type' and 'url' of package repository as listed in package.json
	Repository   Repository      `json:"repository"`
	Homepage     string          `json:"homepage"`
	Keywords     []string        `json:"keywords"`
	Contributors []RemoteUser    `json:"contributors"`
	Users        map[string]bool `json:"users"`
}

func (p Package) GetName() string { return p.Name }

func (p Package) LatestVersion() string {
	if p.DistTags == nil {
		return ""
	}
	return p.DistTags["latest"]
}

type Version struct {
	// Package name
	Name string `json:"name"`
	// Version number
	Version string `json:"version"`
	// Description as listed in package.json
	Description string `json:"description"`
	Main        string `json:"main"`
	// Object with devDependencies and versions as listed in package.json
	DevDependencies map[string]string `json:"devDependencies"`
	// Object with scripts as listed in package.json
	Scripts map[string]string `json:"scripts"`
	// Object with 'name', 'email' and 'url' of the author listed in package.json
	Author RemoteUser `json:"author"`
	// Object containing a 'shasum' and 'tarball' url, usually in the form of https://registry.npmjs.org/<name>/-/<name>-<version>.tgz
	Distribution Distribution `json:"dist"`
	// Array of objects containing author objects as listed in package.json
	Maintainers        []RemoteUser `json:"maintainers"`
	DeprecationMessage string       `json:"deprecated"`
}

func (v Version) GetDeprecated() bool {
	return v.DeprecationMessage != ""
}

type Distribution struct {
	Shasum     string                  `json:"shasum"`
	Tarball    string                  `json:"tarball"`
	Integrity  string                  `json:"integrity"`
	Signatures []DistributionSignature `json:"signatures"`
}

type DistributionSignature struct {
	KeyID     string `json:"keyid"`
	Signature string `json:"sig"`
}

type RemoteUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	URL   string `json:"url"`
}

type Repository struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}
