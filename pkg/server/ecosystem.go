package server

import (
	"net/http"

	"github.com/thepwagner/hedge/pkg/cache"
	"github.com/thepwagner/hedge/pkg/registry"
	"github.com/thepwagner/hedge/pkg/registry/debian"
	"go.opentelemetry.io/otel/trace"
)

func Ecosystems(tracer trace.Tracer, client *http.Client, storage cache.Storage) []registry.EcosystemProvider {
	return []registry.EcosystemProvider{
		debian.NewEcosystemProvider(tracer, client, storage),
	}
}
