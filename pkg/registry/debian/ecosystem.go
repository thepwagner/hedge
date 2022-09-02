package debian

import (
	"net/http"

	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/pkg/registry"
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
