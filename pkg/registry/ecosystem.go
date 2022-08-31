package registry

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/pkg/filter"
	"go.opentelemetry.io/otel/trace"
)

type Ecosystem string

type Package interface {
	GetName() string
}

type EcosystemProvider interface {
	Ecosystem() Ecosystem
	BlankRepositoryConfig() RepositoryConfig

	AllPackages(context.Context, RepositoryConfig) ([]Package, error)

	NewHandler(HandlerArgs) (HasRoutes, error)
}

type HandlerArgs struct {
	Tracer      trace.Tracer
	Client      *http.Client
	ByteStorage cached.ByteStorage
	Ecosystem   EcosystemConfig
}

type RepositoryConfig interface {
	Name() string
	SetName(string)
	FilterConfig() filter.Config
}

// EcosystemConfig is configuration for an ecosystem.
type EcosystemConfig struct {
	Repositories map[string]RepositoryConfig
	Policies     map[string]string
}

type HasRoutes interface {
	Register(*mux.Router)
}
