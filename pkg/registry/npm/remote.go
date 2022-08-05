package npm

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/trace"
)

type RemoteLoader struct {
	tracer  trace.Tracer
	client  *http.Client
	baseURL string
}

var _ PackageLoader = (*RemoteLoader)(nil)

func NewRemoteLoader(tracer trace.Tracer, client *http.Client, baseURL string) *RemoteLoader {
	return &RemoteLoader{
		tracer:  tracer,
		client:  client,
		baseURL: baseURL,
	}
}

func (l *RemoteLoader) GetPackage(ctx context.Context, pkg string) (*Package, error) {
	ctx, span := l.tracer.Start(ctx, "loader.GetPackage")
	defer span.End()

	req, err := http.NewRequest("GET", l.baseURL+pkg, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ParsePackage(resp.Body)
}
