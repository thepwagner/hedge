package debian

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-logr/logr"
	"github.com/thepwagner/hedge/pkg/observability"
)

type RemoteLoader struct {
	log    logr.Logger
	client *http.Client

	baseURL       string
	dist          string
	keyring       openpgp.EntityList
	architectures []string
	components    []string
}

func NewRemoteLoader(log logr.Logger, cfg UpstreamConfig) (*RemoteLoader, error) {
	if cfg.Release == "" {
		return nil, fmt.Errorf("missing release")
	}

	if cfg.Key == "" {
		return nil, fmt.Errorf("missing keyfile")
	}
	kr, err := ReadArmoredKeyRingFile(cfg.Key)
	if err != nil {
		return nil, err
	}

	baseURL := cfg.URL
	if baseURL == "" {
		baseURL = "https://deb.debian.org/debian"
	}
	architectures := cfg.Architectures
	if len(architectures) == 0 {
		architectures = []string{"all", "amd64"}
	}
	components := cfg.Components
	if len(components) == 0 {
		components = []string{"main", "contrib", "non-free"}
	}

	l := &RemoteLoader{
		log:           log.WithName("debian-loader").WithValues("release", cfg.Release),
		baseURL:       baseURL,
		keyring:       kr,
		dist:          cfg.Release,
		client:        http.DefaultClient,
		architectures: architectures,
		components:    components,
	}
	l.log.Info("created remote debian loader", "base_url", l.baseURL, "architectures", l.architectures, "components", l.components)
	return l, nil
}

func (r *RemoteLoader) Load(ctx context.Context) (*Release, error) {
	log := observability.Logger(ctx, r.log).V(1)
	log.Info("loading")

	req, err := http.NewRequestWithContext(ctx, "GET", r.baseURL+"/dists/"+r.dist+"/InRelease", nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Info("fetched", "status", resp.StatusCode, "bytes_count", len(b))

	return ParseReleaseFile(b, ParseReleaseOptions{
		SigningKey:    r.keyring,
		Architectures: r.architectures,
		Components:    r.components,
	})
}
