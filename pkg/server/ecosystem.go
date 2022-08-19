package server

import (
	"github.com/thepwagner/hedge/pkg/registry"
	"github.com/thepwagner/hedge/pkg/registry/debian"
)

var ecosystems = []registry.EcosystemProvider{
	debian.EcosystemProvider{},
}
