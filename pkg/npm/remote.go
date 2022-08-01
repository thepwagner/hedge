package npm

import (
	"context"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

type RemoteLoader struct {
	tracer  trace.Tracer
	client  *http.Client
	baseURL string
}

var _ PackageLoader = (*RemoteLoader)(nil)

func NewRemoteLoader(tp trace.TracerProvider, baseURL string) *RemoteLoader {
	tr := otelhttp.NewTransport(http.DefaultTransport, otelhttp.WithTracerProvider(tp))
	return &RemoteLoader{
		tracer:  tp.Tracer("hedge"),
		baseURL: baseURL,
		client: &http.Client{
			Transport: tr,
		},
	}
}

func (l *RemoteLoader) GetPackage(ctx context.Context, pkg string) (*Package, error) {
	ctx, span := l.tracer.Start(ctx, "npm-loader.GetPackage")
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
