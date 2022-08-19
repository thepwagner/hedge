package registry

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/thepwagner/hedge/pkg/cache"
	"go.opentelemetry.io/otel/trace"
)

type Ecosystem string

type EcosystemProvider interface {
	Ecosystem() Ecosystem
	BlankRepositoryConfig() RepositoryConfig

	NewHandler(HandlerArgs) (HasRoutes, error)
}

type HandlerArgs struct {
	Tracer    trace.Tracer
	Client    *http.Client
	Untrusted cache.Storage
	Trusted   cache.Storage
	Ecosystem EcosystemConfig
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
