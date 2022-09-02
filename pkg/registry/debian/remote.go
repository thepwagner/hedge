package debian

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/proto/hedge/v1"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RemoteRepository struct {
	tracer   trace.Tracer
	fetchURL cached.Function[string, []byte]
	parser   PackageParser
}

func NewRemoteRepository(tracer trace.Tracer, fetchURL cached.Function[string, []byte]) *RemoteRepository {
	return &RemoteRepository{
		tracer:   tracer,
		fetchURL: fetchURL,
		parser:   NewPackageParser(tracer),
	}
}

func (r *RemoteRepository) LoadRelease(ctx context.Context, args LoadReleaseArgs) (*hedge.DebianRelease, error) {
	ctx, span := r.tracer.Start(ctx, "debianremote.release.Load")
	defer span.End()

	u, err := url.JoinPath(args.MirrorURL, "dists", args.Dist, "InRelease")
	if err != nil {
		return nil, fmt.Errorf("building URL: %w", err)
	}
	b, err := r.fetchURL(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("fetching release file: %w", err)
	}

	key, err := openpgp.ReadArmoredKeyRing(strings.NewReader(args.SigningKey))
	if err != nil {
		return nil, fmt.Errorf("reading key: %w", err)
	}
	graph, err := ParseReleaseFile(b, key)
	if err != nil {
		return nil, fmt.Errorf("parsing release file: %w", err)
	}

	ret := hedge.DebianRelease{
		MirrorUrl:     args.MirrorURL,
		Dist:          args.Dist,
		AcquireByHash: graph["Acquire-By-Hash"] == "yes",
		Architectures: strings.Split(graph["Architectures"], " "),
	}
	for k, v := range graph {
		switch k {
		case "Acquire-By-Hash":
		case "Architectures":
			ret.Architectures = strings.Split(v, " ")
		case "Changelogs":
			ret.Changelogs = v
		case "Codename":
			ret.Codename = v
		case "Components":
			ret.Components = strings.Split(v, " ")
		case "Date":
			t, err := time.Parse(time.RFC1123, v)
			if err != nil {
				return nil, fmt.Errorf("parsing date: %w", err)
			}
			ret.Date = timestamppb.New(t)
		case "Description":
			ret.Description = v
		case "Label":
			ret.Label = v
		case "MD5Sum", "SHA256":
			// skipped, as these are calculated below
		case "No-Support-for-Architecture-all":
			ret.NoSupportForArchitectureAll = v == "yes"
		case "Origin":
			ret.Origin = v
		case "Suite":
			ret.Suite = v
		case "Version":
			ret.Version = v
		default:
			return nil, fmt.Errorf("unknown key: %s", k)
		}
	}

	digests, err := parseDigests(graph)
	if err != nil {
		return nil, err
	}
	ret.Digests = digests

	return &ret, nil
}

func (r *RemoteRepository) LoadPackages(ctx context.Context, args LoadPackagesArgs) (*hedge.DebianPackages, error) {
	arch := args.Architecture
	release := args.Release
	components := release.Components
	ctx, span := r.tracer.Start(ctx, "debianremote.LoadPackages", trace.WithAttributes(attrArchitecture(arch), attrComponents(components)))
	defer span.End()

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(4)
	res := make(chan []*hedge.DebianPackage)
	for _, c := range components {
		component := c
		eg.Go(func() error {
			ctx, span := r.tracer.Start(ctx, "debianremote.LoadPackages.component", trace.WithAttributes(attrComponent(component)))
			defer span.End()

			// The Release file specifies the expected properties of the Packages file
			// The Release file's signature was verified, so we trust it.
			fn := fmt.Sprintf("%s/binary-%s/Packages.gz", component, arch)
			digest, ok := args.Release.Digests[fn]
			if !ok {
				return fmt.Errorf("release is missing %s/%s", component, arch)
			}
			u, err := url.JoinPath(release.MirrorUrl, "dists", release.Dist, digest.Path)
			if err != nil {
				return fmt.Errorf("building URL: %w", err)
			}

			// If the URL is content-addressed, we can cache it ~forever
			var fetchCtx context.Context
			if strings.Contains(digest.Path, "/by-hash/") {
				fetchCtx = cached.For(ctx, 7*24*time.Hour)
			} else {
				fetchCtx = ctx
			}
			b, err := r.fetchURL(fetchCtx, u)
			if err != nil {
				return fmt.Errorf("fetching release file: %w", err)
			}

			// Verify the file matches expectations:
			if uint64(len(b)) != digest.Size {
				return fmt.Errorf("expected %d bytes, got %d", digest.Size, len(b))
			}
			if actualDigest := sha256.Sum256(b); !bytes.Equal(actualDigest[:], digest.Sha256Sum) {
				return fmt.Errorf("expected digest %x, got %x", digest.Sha256Sum, actualDigest)
			}

			// Parse packages from verified file:
			gzr, err := gzip.NewReader(bytes.NewReader(b))
			if err != nil {
				return err
			}
			pkgs, err := r.parser.ParsePackages(ctx, gzr)
			if err != nil {
				return err
			}
			res <- pkgs
			return nil
		})
	}
	go func() {
		_ = eg.Wait()
		close(res)
	}()

	var allPackages []*hedge.DebianPackage
	for pkgs := range res {
		allPackages = append(allPackages, pkgs...)
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return &hedge.DebianPackages{
		Packages: allPackages,
	}, nil
}
