package registry

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/trace"
)

type Ecosystem string

type EcosystemProvider interface {
	Ecosystem() Ecosystem
	BlankRepositoryConfig() RepositoryConfig

	NewHandler(tracer trace.Tracer, client *http.Client, ecoCfg EcosystemConfig) (HasRoutes, error)
}

type RepositoryConfig interface {
	Name() string
	SetName(string)
	PolicyNames() []string
}

// EcosystemConfig is configuration for an ecosystem.
type EcosystemConfig struct {
	Repositories map[string]RepositoryConfig
	Policies     map[string]string
}

type HasRoutes interface {
	Register(*mux.Router)
}
