package npm

import (
	"net/http"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

type RemoteLoader struct {
	log    logr.Logger
	tracer trace.Tracer
	client *http.Client
}

func NewRemoteLoader(log logr.Logger, tp trace.TracerProvider) *RemoteLoader {
	tr := otelhttp.NewTransport(http.DefaultTransport, otelhttp.WithTracerProvider(tp))
	return &RemoteLoader{
		log:    log,
		tracer: tp.Tracer("hedge"),
		client: &http.Client{
			Transport: tr,
		},
	}
}

type RemotePackage struct {
	ID           string                       `json:"_id"`
	Name         string                       `json:"name"`
	Description  string                       `json:"description"`
	DistTags     map[string]string            `json:"dist-tags"`
	Versions     map[string]RemoteVersionInfo `json:"versions"`
	Readme       string                       `json:"readme"`
	Maintainers  []RemoteUser                 `json:"maintainers"`
	Times        map[string]string            `json:"time"`
	Author       RemoteUser                   `json:"author"`
	Homepage     string                       `json:"homepage"`
	Keywords     []string                     `json:"keywords"`
	Contributors []RemoteUser                 `json:"contributors"`
	Users        map[string]bool              `json:"users"`
}

type RemoteVersionInfo struct {
	Name            string             `json:"name"`
	Version         string             `json:"version"`
	Description     string             `json:"description"`
	Main            string             `json:"main"`
	DevDependencies map[string]string  `json:"devDependencies"`
	Scripts         map[string]string  `json:"scripts"`
	Author          RemoteUser         `json:"author"`
	Distribution    RemoteDistribution `json:"dist"`
	Maintainers     []RemoteUser       `json:"maintainers"`
	Deprecated      bool               `json:"deprecated"`
}

type RemoteDistribution struct {
	Shasum     string            `json:"shasum"`
	Tarball    string            `json:"tarball"`
	Integrity  string            `json:"integrity"`
	Signatures []RemoteSignature `json:"signatures"`
}

type RemoteSignature struct {
	KeyID     string `json:"keyid"`
	Signature string `json:"sig"`
}

type RemoteUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
