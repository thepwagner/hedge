package server

import (
	"net/http"

	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/pkg/registry"
	"github.com/thepwagner/hedge/pkg/registry/debian"
	"go.opentelemetry.io/otel/trace"
)

func Ecosystems(tracer trace.Tracer, client *http.Client, storage cached.ByteStorage) []registry.EcosystemProvider {
	return []registry.EcosystemProvider{
		debian.NewEcosystemProvider(tracer, client, storage),
	}
}
