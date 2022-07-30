package debian

import (
	"context"
	"io/ioutil"
	"net/http"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-logr/logr"
)

type RemoteLoader struct {
	log      logr.Logger
	baseURL  string
	keyrings map[string]openpgp.EntityList
	client   *http.Client

	architectures []string
	components    []string
}

func NewRemoteLoader(log logr.Logger, opts ...RemoteLoaderOption) *RemoteLoader {
	l := &RemoteLoader{
		log:      log.WithName("debian-loader"),
		baseURL:  "https://deb.debian.org/debian",
		keyrings: make(map[string]openpgp.EntityList),
		client:   http.DefaultClient,
	}
	for _, opt := range opts {
		opt(l)
	}

	if len(l.architectures) == 0 {
		l.architectures = []string{"all", "amd64"}
	}
	if len(l.components) == 0 {
		l.components = []string{"main", "contrib", "non-free"}
	}

	l.log.Info("creating remote debian loader", "base_url", l.baseURL, "architectures", l.architectures, "components", l.components)
	return l
}

type RemoteLoaderOption func(*RemoteLoader)

func WithRemoteLoaderBaseURL(baseURL string) RemoteLoaderOption {
	return func(r *RemoteLoader) {
		r.baseURL = baseURL
	}
}

func (r *RemoteLoader) AddKeyring(dist string, keyring openpgp.EntityList) {
	r.keyrings[dist] = keyring
}

func (r *RemoteLoader) Load(ctx context.Context, dist string) (*Release, error) {
	log := r.log.WithValues("dist", dist)
	keyring, ok := r.keyrings[dist]
	if !ok {
		log.Info("no keyring found, skipping")
		return nil, nil
	}
	log.Info("loading")

	req, err := http.NewRequestWithContext(ctx, "GET", r.baseURL+"/dists/"+dist+"/InRelease", nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	log.Info("fetched", "status", resp.StatusCode)

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return ParseReleaseFile(b, ParseReleaseOptions{
		SigningKey:    keyring,
		Architectures: r.architectures,
		Components:    r.components,
	})
}
