package debian

import (
	"net/http"

	"github.com/thepwagner/hedge/pkg/registry"
	"github.com/thepwagner/hedge/pkg/registry/base"
	"go.opentelemetry.io/otel/trace"
)

const Ecosystem registry.Ecosystem = "debian"

type EcosystemProvider struct{}

var _ registry.EcosystemProvider = (*EcosystemProvider)(nil)

func (e EcosystemProvider) Ecosystem() registry.Ecosystem { return Ecosystem }
func (e EcosystemProvider) BlankRepositoryConfig() registry.RepositoryConfig {
	return &RepositoryConfig{}
}

func (e EcosystemProvider) NewHandler(tracer trace.Tracer, client *http.Client, ecoCfg registry.EcosystemConfig) (registry.HasRoutes, error) {
	h, err := base.NewHandler(tracer, ecoCfg, func(cfg *RepositoryConfig) (*repositoryHandler, error) {
		return newRepositoryHandler(tracer, client, ecoCfg.Policies, cfg)
	})
	if err != nil {
		return nil, err
	}
	return &Handler{Handler: *h}, nil
}
