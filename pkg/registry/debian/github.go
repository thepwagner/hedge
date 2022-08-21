package debian

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/thepwagner/hedge/pkg/registry"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type FixedReleaseLoader struct {
	release Release
}

func (r FixedReleaseLoader) Load(context.Context) (*Release, error) {
	release := r.release
	release.DateRaw = time.Now().UTC().Format(time.RFC1123)
	return &release, nil
}

type GitHubPackagesLoader struct {
	tracer trace.Tracer
	client *http.Client
	github *github.Client
	parser PackageParser

	ghRepos []githubRepoConfig
}

func NewGitHubPackagesLoader(tracer trace.Tracer, client *http.Client, cfg GitHubConfig) GitHubPackagesLoader {
	ghRepos := make([]githubRepoConfig, 0, len(cfg.Repositories))
	for _, repo := range cfg.Repositories {
		nwo := strings.SplitN(repo, "/", 2)
		ghRepos = append(ghRepos, githubRepoConfig{
			owner: nwo[0],
			name:  nwo[1],
		})
	}

	// TODO: auth goes here
	gh := github.NewClient(client)

	return GitHubPackagesLoader{
		tracer:  tracer,
		parser:  NewPackageParser(tracer),
		client:  client,
		github:  gh,
		ghRepos: ghRepos,
	}
}

type githubRepoConfig struct {
	owner, name string
}

func (gh GitHubPackagesLoader) BaseURL() string { return "https://github.com/" }

func (gh GitHubPackagesLoader) LoadPackages(ctx context.Context, arch Architecture) ([]registry.Package, error) {
	ctx, span := gh.tracer.Start(ctx, "githubloader.LoadPackages")
	defer span.End()

	archRE := regexp.MustCompile(fmt.Sprintf("[-_]%s\\.deb$", arch))
	var packages []registry.Package
	for _, repo := range gh.ghRepos {
		releases, _, err := gh.github.Repositories.ListReleases(ctx, repo.owner, repo.name, nil)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("listing releases for %s/%s: %w", repo.owner, repo.name, err)
		}
		for _, r := range releases {
			for _, asset := range r.Assets {
				if archRE.MatchString(asset.GetName()) {
					debURL := asset.GetBrowserDownloadURL()
					req, err := http.NewRequest("GET", debURL, nil)
					if err != nil {
						span.RecordError(err)
						return nil, fmt.Errorf("creating request for %s: %w", debURL, err)
					}
					req = req.WithContext(ctx)
					resp, err := gh.client.Do(req)
					if err != nil {
						span.RecordError(err)
						return nil, fmt.Errorf("downloading %s: %w", debURL, err)
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						span.SetStatus(codes.Error, fmt.Sprintf("downloading %s: %s", debURL, resp.Status))
						return nil, fmt.Errorf("downloading %s: %s", debURL, resp.Status)
					}

					pkgData, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						span.RecordError(err)
						return nil, fmt.Errorf("reading %s: %w", debURL, err)
					}

					pkg, err := gh.parser.PackageFromDeb(ctx, bytes.NewReader(pkgData))
					if err != nil {
						span.RecordError(err)
						return nil, fmt.Errorf("parsing %s: %w", debURL, err)
					}

					if pkg != nil {
						pkg.MD5Sum = fmt.Sprintf("%x", md5.Sum(pkgData))
						pkg.Sha256 = fmt.Sprintf("%x", sha256.Sum256(pkgData))
						pkg.SizeRaw = fmt.Sprintf("%d", len(pkgData))
						// FIXME: dist is hack
						pkg.Filename = fmt.Sprintf("dists/github/pool/%s", strings.TrimPrefix(debURL, "https://github.com/"))

						packages = append(packages, *pkg)
					}
				}
			}
			break
		}
	}

	return packages, nil
}
