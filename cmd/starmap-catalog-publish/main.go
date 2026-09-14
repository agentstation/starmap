// Command starmap-catalog-publish prepares catalog artifacts and replay checkpoints.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/internal/catalog/publication"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/internal/providers/clients"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type prepareOptions struct{ profile, state, stateChecksum, publisher, output, runID, workspace string }

type preparedRecord struct {
	RequestChecksum string                    `json:"request_checksum"`
	State           publication.ReceiptRecord `json:"state"`
	Receipt         publication.ReceiptRecord `json:"receipt"`
	ReusedArtifact  bool                      `json:"reused_artifact"`
}

type prepareReport struct {
	Status            string `json:"status"`
	GenerationID      string `json:"generation_id"`
	ArchiveChecksum   string `json:"archive_checksum"`
	ArtifactDirectory string `json:"artifact_directory"`
	ReceiptPath       string `json:"receipt_path"`
	ReceiptChecksum   string `json:"receipt_checksum"`
	StatePath         string `json:"state_path"`
	StateChecksum     string `json:"state_checksum"`
	ReusedArtifact    bool   `json:"reused_artifact"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil && !stderrors.Is(err, flag.ErrHelp) {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("starmap-catalog-publish", flag.ContinueOnError)
	flags.SetOutput(output)
	var options prepareOptions
	flags.StringVar(&options.profile, "profile", "", "explicit publication source policy YAML")
	flags.StringVar(&options.state, "state", "", "previous accepted publication checkpoint")
	flags.StringVar(&options.stateChecksum, "state-checksum", "", "trusted checksum of the accepted checkpoint")
	flags.StringVar(&options.publisher, "publisher-id", "", "stable identity that owns this publisher's source scopes")
	flags.StringVar(&options.output, "output-dir", "", "private directory for prepared publication outputs")
	flags.StringVar(&options.runID, "run-id", "", "stable run identity reused by retries")
	flags.StringVar(&options.workspace, "workspace", "", "explicit local source workspace")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || options.profile == "" || options.publisher == "" || options.output == "" || options.runID == "" {
		return invalid("arguments", "requires profile, publisher-id, output-dir, and run-id without positional arguments")
	}
	if (options.state == "") != (options.stateChecksum == "") {
		return invalid("state", "state and state-checksum must be supplied together")
	}
	record, err := prepare(ctx, options)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(record)
}

func prepare(ctx context.Context, options prepareOptions) (prepareReport, error) {
	if err := ctx.Err(); err != nil {
		return prepareReport{}, err
	}
	raw, err := readBounded(options.profile, publication.MaxPublicationProfileBytes)
	if err != nil {
		return prepareReport{}, err
	}
	profile, err := publication.ParseProfile(raw)
	if err != nil {
		return prepareReport{}, err
	}
	var state *publication.State
	if options.state != "" {
		raw, err := readBounded(options.state, publication.MaxPublicationStateBytes)
		if err != nil {
			return prepareReport{}, err
		}
		state, err = publication.RestoreState(ctx, raw, options.stateChecksum)
		if err != nil {
			return prepareReport{}, err
		}
	} else {
		baseline, err := bootstrap.Generation()
		if err != nil {
			return prepareReport{}, err
		}
		state, err = publication.NewState(baseline, options.publisher)
		if err != nil {
			return prepareReport{}, err
		}
	}
	if state.PublisherID() != options.publisher {
		return prepareReport{}, invalid("publisher_id", "does not match the accepted checkpoint")
	}
	initial, err := publication.EncodeState(state)
	if err != nil {
		return prepareReport{}, err
	}
	requestBytes, err := json.Marshal(struct{ RunID, Publisher, Profile, State, Workspace string }{options.runID, options.publisher, digest(raw), initial.Checksum, options.workspace})
	if err != nil {
		return prepareReport{}, err
	}
	requestChecksum := digest(requestBytes)
	absolute, err := filepath.Abs(options.output)
	if err != nil {
		return prepareReport{}, err
	}
	root, err := privatefiles.NewDirectory(absolute)
	if err != nil {
		return prepareReport{}, err
	}
	cacheName := "prepared-" + strings.TrimPrefix(digest([]byte(options.runID)), "sha256:") + ".json"
	data, err := root.ReadFile(cacheName, publication.MaxPublicationStateBytes*2)
	if os.IsNotExist(err) {
		producer, err := publication.NewProducer(profile, func(provider *catalogs.Provider) (sources.ProviderClient, error) {
			return clients.NewProvider(provider)
		}, auth.NewResolver())
		if err != nil {
			return prepareReport{}, err
		}
		var sourceOptions []pkgsync.Option
		if options.workspace != "" {
			sourceOptions = append(sourceOptions, pkgsync.WithCatalogPath(options.workspace))
		}
		candidate, err := producer.Prepare(ctx, state, options.runID, sourceOptions...)
		if err != nil {
			if len(candidate.Decision.Scopes) != 0 && !candidate.Decision.Allowed {
				path, recordErr := recordRejection(ctx, root, absolute, requestChecksum, candidate.Decision)
				if recordErr != nil {
					return prepareReport{}, stderrors.Join(err, recordErr)
				}
				return prepareReport{}, pkgerrors.WrapResource("admit", "publication", path, err)
			}
			return prepareReport{}, err
		}
		checkpoint, err := publication.EncodeState(candidate.Next)
		if err != nil {
			return prepareReport{}, err
		}
		data, err = json.Marshal(preparedRecord{RequestChecksum: requestChecksum, State: checkpoint, Receipt: candidate.Receipt, ReusedArtifact: candidate.ReusedArtifact})
		if err != nil {
			return prepareReport{}, err
		}
		if err := root.WriteFileIfAbsentContext(ctx, cacheName, data, "publication-prepare"); err != nil {
			// A concurrent writer or ambiguous flush can leave the selected candidate visible.
			accepted, readErr := root.ReadFile(cacheName, publication.MaxPublicationStateBytes*2)
			if readErr != nil {
				return prepareReport{}, err
			}
			data = accepted
		}
	} else if err != nil {
		return prepareReport{}, err
	}
	var selected preparedRecord
	if err := json.Unmarshal(data, &selected); err != nil {
		return prepareReport{}, invalid("prepared", "contains invalid retained preparation")
	}
	if selected.RequestChecksum != requestChecksum {
		return prepareReport{}, invalid("run_id", "already belongs to a different publication request")
	}
	return stagePrepared(ctx, absolute, selected)
}

func recordRejection(ctx context.Context, root *privatefiles.Directory, directory, requestChecksum string, decision publication.Decision) (string, error) {
	data, err := json.Marshal(struct {
		RequestChecksum string               `json:"request_checksum"`
		Decision        publication.Decision `json:"decision"`
	}{requestChecksum, decision})
	if err != nil {
		return "", err
	}
	name := "rejected-" + strings.TrimPrefix(digest(data), "sha256:") + ".json"
	if err := root.WriteFileIfAbsentContext(ctx, name, data, "publication-rejection"); err != nil {
		prior, readErr := root.ReadFile(name, publication.MaxPublicationProfileBytes)
		if readErr != nil || digest(prior) != digest(data) {
			return "", err
		}
	}
	return filepath.Join(directory, name), ctx.Err()
}

func stagePrepared(ctx context.Context, output string, selected preparedRecord) (prepareReport, error) {
	state, err := publication.RestoreState(ctx, selected.State.Data, selected.State.Checksum)
	if err != nil {
		return prepareReport{}, err
	}
	generation := state.Generation()
	bundle, err := artifact.Build(generation)
	if err != nil {
		return prepareReport{}, err
	}
	semantic, err := generation.SemanticChecksum()
	if err != nil {
		return prepareReport{}, err
	}
	expected := artifact.PublicationArtifact{GenerationID: generation.Manifest.GenerationID, CatalogChecksum: semantic, PayloadChecksum: generation.Manifest.Payload.Checksum, ArchiveChecksum: bundle.Checksum}
	if _, err := artifact.VerifyPublicationReceipt(selected.Receipt.Data, selected.Receipt.Checksum, expected); err != nil {
		return prepareReport{}, err
	}
	assets, err := artifact.StageReleaseAssets(filepath.Join(output, "artifacts"), bundle)
	if err != nil {
		return prepareReport{}, err
	}
	root, err := privatefiles.NewDirectory(output)
	if err != nil {
		return prepareReport{}, err
	}
	stateName := "state-" + strings.TrimPrefix(selected.State.Checksum, "sha256:") + ".json"
	receiptName := "receipt-" + strings.TrimPrefix(selected.Receipt.Checksum, "sha256:") + ".json"
	if err := root.WriteFileIfAbsentContext(ctx, stateName, selected.State.Data, "publication-state"); err != nil {
		data, readErr := root.ReadFile(stateName, publication.MaxPublicationStateBytes)
		if readErr != nil || digest(data) != selected.State.Checksum {
			return prepareReport{}, err
		}
	}
	if err := root.WriteFileIfAbsentContext(ctx, receiptName, selected.Receipt.Data, "publication-receipt"); err != nil {
		data, readErr := root.ReadFile(receiptName, publication.MaxPublicationProfileBytes)
		if readErr != nil || digest(data) != selected.Receipt.Checksum {
			return prepareReport{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return prepareReport{}, err
	}
	return prepareReport{Status: "prepared", GenerationID: generation.Manifest.GenerationID, ArchiveChecksum: bundle.Checksum, ArtifactDirectory: assets.Directory, ReceiptPath: filepath.Join(output, receiptName), ReceiptChecksum: selected.Receipt.Checksum, StatePath: filepath.Join(output, stateName), StateChecksum: selected.State.Checksum, ReusedArtifact: selected.ReusedArtifact}, nil
}

func readBounded(path string, limit int64) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	file, err := root.Open(filepath.Base(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, invalid("input", "requires a bounded regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, invalid("input", "exceeds the input byte bound")
	}
	return data, nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func invalid(field, message string) error {
	return &pkgerrors.ValidationError{Field: "publication." + field, Message: message}
}
