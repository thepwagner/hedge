package debian

import (
	"net/http"

	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/pkg/registry"
	"github.com/thepwagner/hedge/pkg/registry/base"
	"go.opentelemetry.io/otel/trace"
)

const Ecosystem registry.Ecosystem = "debian"

type EcosystemProvider struct {
	tracer  trace.Tracer
	client  *http.Client
	storage cached.ByteStorage
}

func NewEcosystemProvider(tracer trace.Tracer, client *http.Client, storage cached.ByteStorage) *EcosystemProvider {
	return &EcosystemProvider{
		tracer:  tracer,
		client:  client,
		storage: storage,
	}
}

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
