package runtime

import (
	"context"
	"os"
	"sync"

	"github.com/agentstation/starmap/internal/sources/github"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// maxSourceFileBytes bounds a catalog payload read from a local file. A larger
// file is unsafe input, so the source rejects it before it decodes anything.
const maxSourceFileBytes = 64 << 20

// selectSource builds the configured upstream source. The choice is terminal.
// A deployment that names a source other than the public channel never falls
// back to public GitHub. A misconfiguration fails instead of sending traffic.
func (r *Runtime) selectSource() (Source, error) {
	if r.config.customSource != nil {
		return r.config.customSource, nil
	}
	switch r.config.source.Kind {
	case SourceEmbedded:
		return embeddedSource{}, nil
	case SourcePublic, SourceGitHub:
		return r.newGitHubSource()
	case SourceFile:
		return newFileSource(r.config.source)
	case SourceStarmap:
		return nil, &errors.ConfigError{
			Component: "catalog source",
			Message: "the starmap cascade source arrives through WithSource, " +
				"because the root package cannot import the cascade subscriber",
		}
	default:
		return nil, &errors.ValidationError{
			Field:   "catalog_source",
			Value:   string(r.config.source.Kind),
			Message: "names no known source",
		}
	}
}

// newGitHubSource builds the attested GitHub channel source. It maps the
// configured transfer bounds onto the source transport and shares the runtime
// state directory, so discovery state survives a restart.
func (r *Runtime) newGitHubSource() (Source, error) {
	if r.config.stateDirectory == "" {
		return nil, &errors.ConfigError{
			Component: "catalog source",
			Message:   "the GitHub catalog source needs a state directory",
		}
	}
	opts := []github.Option{
		github.WithRepository(r.config.source.Repository),
		github.WithChannel(r.config.source.Channel),
		github.WithStateDirectory(r.config.stateDirectory),
		github.WithTransferPolicy(r.config.transferPolicy()),
		github.WithClock(r.config.now),
	}
	if workflow := r.config.source.SignerWorkflow; workflow != "" {
		opts = append(opts, github.WithSignerWorkflow(workflow))
	}
	if token := r.config.sourceToken; token != "" {
		opts = append(opts, github.WithToken(token))
	}
	if url := r.config.source.URL; url != "" {
		opts = append(opts, github.WithAPIBaseURL(url))
	}
	source, err := github.New(opts...)
	if err != nil {
		return nil, err
	}
	return &githubSource{source: source, identity: r.config.source.SafeIdentity()}, nil
}

// githubSource adapts the attested GitHub channel onto the runtime source role.
type githubSource struct {
	source *github.Source

	// identity is the safe identity of the configured channel. It names the
	// repository and the channel, never a credential.
	identity string
}

// Identity returns the safe identity of the configured GitHub channel.
func (g *githubSource) Identity() string { return g.identity }

// Read checks the channel first and downloads only after a change. The
// conditional check keeps the request budget small.
func (g *githubSource) Read(ctx context.Context) (SourceRead, error) {
	status, err := g.source.Changed(ctx)
	if err != nil {
		return SourceRead{}, err
	}
	if status.Budget.Warn() {
		// The check reports the budget on success too, so an operator sees the
		// warning before a refusal arrives.
		logging.Warn().
			Int("used_percent", status.Budget.UsedPercent()).
			Str("source", g.identity).
			Msg("Catalog source request budget is nearly spent")
	}
	read := SourceRead{Health: HealthOK}
	if !status.Changed {
		return read, nil
	}
	release, err := g.source.ReadChannel(ctx)
	if err != nil {
		return SourceRead{}, err
	}
	read.Changed = true
	read.Generation = release.Generation
	read.PublishedAt = release.PublishedAt
	read.ChannelUpdatedAt = release.PublishedAt
	return read, nil
}

// embeddedSource selects the verified embedded catalog. It reads nothing, so a
// deployment that pins the embedded catalog reaches no external system at all.
type embeddedSource struct{}

// Identity returns the safe identity of the embedded catalog.
func (embeddedSource) Identity() string { return string(SourceEmbedded) }

// Read reports no change, because the embedded catalog moves only with a new
// binary.
func (embeddedSource) Read(context.Context) (SourceRead, error) {
	return SourceRead{Health: HealthOK}, nil
}

// fileSource reads one catalog payload from a local path. A deployment uses it
// to serve a catalog that an operator placed on disk.
type fileSource struct {
	path string

	mu       sync.Mutex
	checksum string
}

// newFileSource builds the local file source.
func newFileSource(policy SourcePolicy) (Source, error) {
	if policy.URL == "" {
		return nil, &errors.ConfigError{
			Component: "catalog source",
			Message:   "the file catalog source needs a path",
		}
	}
	return &fileSource{path: policy.URL}, nil
}

// Identity returns the safe identity of the file source. It names no path.
func (f *fileSource) Identity() string { return string(SourceFile) }

// Read decodes the payload and reports a change when its digest moved.
func (f *fileSource) Read(_ context.Context) (SourceRead, error) {
	info, err := os.Stat(f.path)
	if err != nil {
		return SourceRead{}, errors.WrapIO("stat", f.path, err)
	}
	if info.Size() > maxSourceFileBytes {
		return SourceRead{}, &errors.ResourceError{
			Operation: "read",
			Resource:  "file catalog source",
			ID:        f.path,
			Err: &errors.ValidationError{
				Field: "payload_bytes", Value: info.Size(), Message: "exceeds the payload bound",
			},
		}
	}
	payload, err := os.ReadFile(f.path) //nolint:gosec // The path is operator-configured state.
	if err != nil {
		return SourceRead{}, errors.WrapIO("read", f.path, err)
	}
	descriptor := catalogs.DescribeCatalogPayload(payload)

	f.mu.Lock()
	unchanged := descriptor.Checksum == f.checksum
	f.mu.Unlock()
	if unchanged {
		return SourceRead{Health: HealthOK}, nil
	}
	if _, err := catalogs.DecodeCatalogPayload(payload); err != nil {
		return SourceRead{}, errors.WrapResource("decode", "file catalog source", f.path, err)
	}

	f.mu.Lock()
	f.checksum = descriptor.Checksum
	f.mu.Unlock()
	return SourceRead{
		Changed: true,
		Generation: catalogs.Generation{
			Manifest: catalogs.GenerationManifest{
				GenerationID: descriptor.Checksum,
				GeneratedAt:  info.ModTime().UTC(),
				Payload:      descriptor,
			},
			Payload: payload,
		},
		PublishedAt:      info.ModTime().UTC(),
		ChannelUpdatedAt: info.ModTime().UTC(),
		Health:           HealthOK,
	}, nil
}
