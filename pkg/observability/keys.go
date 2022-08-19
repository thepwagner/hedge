package observability

import (
	"github.com/thepwagner/hedge/pkg/registry"
	"go.opentelemetry.io/otel/attribute"
)

var (
	ecosystemKey      = attribute.Key("ecosystem")
	repositoryNameKey = attribute.Key("repository_name")
)

func Ecosystem(e registry.Ecosystem) attribute.KeyValue {
	return ecosystemKey.String(string(e))
}

func RepositoryName(rn string) attribute.KeyValue {
	return repositoryNameKey.String(rn)
}
