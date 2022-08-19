package debian

import (
	"github.com/thepwagner/hedge/pkg/registry"
	"github.com/thepwagner/hedge/pkg/registry/base"
)

const Ecosystem registry.Ecosystem = "debian"

type EcosystemProvider struct{}

var _ registry.EcosystemProvider = (*EcosystemProvider)(nil)

func (e EcosystemProvider) Ecosystem() registry.Ecosystem { return Ecosystem }
func (e EcosystemProvider) BlankRepositoryConfig() registry.RepositoryConfig {
	return &RepositoryConfig{}
}

func (e EcosystemProvider) NewHandler(args registry.HandlerArgs) (registry.HasRoutes, error) {
	h, err := base.NewHandler(args, func(cfg *RepositoryConfig) (*repositoryHandler, error) {
		return newRepositoryHandler(args, cfg)
	})
	if err != nil {
		return nil, err
	}
	return &Handler{Handler: *h}, nil
}
