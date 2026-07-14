package catalogstore

import (
	"context"
	"database/sql"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	createGenerationsTable = `CREATE TABLE IF NOT EXISTS catalog_generations (
generation_id TEXT PRIMARY KEY,
manifest BLOB NOT NULL,
payload BLOB NOT NULL
)`
	createCurrentTable = `CREATE TABLE IF NOT EXISTS catalog_current (
singleton INTEGER PRIMARY KEY,
generation_id TEXT NOT NULL
)`
)

// SQL stores generations transactionally through database/sql. The baseline
// queries use the common SQLite-style question-mark bind syntax; the supported
// production adapter is SQLite.
type SQL struct {
	db           *sql.DB
	beforeCommit func() error
}

// NewSQL initializes a SQL catalog store.
func NewSQL(ctx context.Context, db *sql.DB) (*SQL, error) {
	if db == nil {
		return nil, &errors.ConfigError{Component: catalogStoreComponent, Message: "SQL database is required"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, statement := range []string{createGenerationsTable, createCurrentTable} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return nil, errors.WrapResource("initialize", "catalog store schema", "", err)
		}
	}
	return &SQL{db: db}, nil
}

// Current returns the currently active generation.
func (s *SQL) Current(ctx context.Context) (Generation, error) {
	if err := ctx.Err(); err != nil {
		return Generation{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT g.manifest, g.payload
FROM catalog_current c
JOIN catalog_generations g ON g.generation_id = c.generation_id
WHERE c.singleton = 1`)
	generation, err := scanGeneration(row)
	if err == sql.ErrNoRows {
		return Generation{}, currentNotFound()
	}
	if err != nil {
		return Generation{}, errors.WrapResource("read", catalogGenerationResource, currentFilename, err)
	}
	return generation, nil
}

// Get returns an immutable generation by ID.
func (s *SQL) Get(ctx context.Context, id string) (Generation, error) {
	if err := ctx.Err(); err != nil {
		return Generation{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT manifest, payload FROM catalog_generations WHERE generation_id = ?`, id)
	generation, err := scanGeneration(row)
	if err == sql.ErrNoRows {
		return Generation{}, generationNotFound(id)
	}
	if err != nil {
		return Generation{}, errors.WrapResource("read", catalogGenerationResource, id, err)
	}
	return generation, nil
}

// Commit transactionally persists and activates generation when the current ID
// matches expectedGenerationID.
func (s *SQL) Commit(ctx context.Context, generation Generation, expectedGenerationID string) error {
	if err := validateCandidate(ctx, generation); err != nil {
		return err
	}
	candidate := generation.Copy()
	manifestData, err := marshalManifest(candidate.Manifest)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return errors.WrapResource("begin", "catalog store transaction", "", err)
	}
	defer func() { _ = tx.Rollback() }()

	currentID, err := sqlCurrentID(ctx, tx)
	if err != nil {
		return err
	}
	existing, found, err := sqlGeneration(ctx, tx, candidate.Manifest.GenerationID)
	if err != nil {
		return err
	}
	if found {
		if !sameGeneration(existing, candidate) {
			return identityConflict(candidate.Manifest.GenerationID)
		}
		if currentID == candidate.Manifest.GenerationID {
			return nil
		}
	}
	if currentID != expectedGenerationID {
		return casConflict(expectedGenerationID, currentID)
	}
	if !found {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO catalog_generations (generation_id, manifest, payload) VALUES (?, ?, ?)`,
			candidate.Manifest.GenerationID, manifestData, candidate.Payload,
		); err != nil {
			return errors.WrapResource("write", catalogGenerationResource, candidate.Manifest.GenerationID, err)
		}
	}
	if currentID == "" {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO catalog_current (singleton, generation_id) VALUES (1, ?)`,
			candidate.Manifest.GenerationID,
		); err != nil {
			return errors.WrapResource("activate", catalogGenerationResource, candidate.Manifest.GenerationID, err)
		}
	} else if _, err := tx.ExecContext(ctx,
		`UPDATE catalog_current SET generation_id = ? WHERE singleton = 1`,
		candidate.Manifest.GenerationID,
	); err != nil {
		return errors.WrapResource("activate", catalogGenerationResource, candidate.Manifest.GenerationID, err)
	}
	if s.beforeCommit != nil {
		if err := s.beforeCommit(); err != nil {
			return errors.WrapResource("commit", "catalog store transaction", candidate.Manifest.GenerationID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return errors.WrapResource("commit", "catalog store transaction", candidate.Manifest.GenerationID, err)
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanGeneration(row rowScanner) (Generation, error) {
	var manifestData, payload []byte
	if err := row.Scan(&manifestData, &payload); err != nil {
		return Generation{}, err
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestData)
	if err != nil {
		return Generation{}, err
	}
	generation := Generation{Manifest: manifest, Payload: append([]byte(nil), payload...)}
	if err := generation.Validate(); err != nil {
		return Generation{}, err
	}
	return generation, nil
}

func sqlCurrentID(ctx context.Context, tx *sql.Tx) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT generation_id FROM catalog_current WHERE singleton = 1`).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", errors.WrapResource("read", "catalog current generation", "", err)
	}
	return id, nil
}

func sqlGeneration(ctx context.Context, tx *sql.Tx, id string) (Generation, bool, error) {
	generation, err := scanGeneration(tx.QueryRowContext(ctx,
		`SELECT manifest, payload FROM catalog_generations WHERE generation_id = ?`, id,
	))
	if err == sql.ErrNoRows {
		return Generation{}, false, nil
	}
	if err != nil {
		return Generation{}, false, errors.WrapResource("read", catalogGenerationResource, id, err)
	}
	return generation, true, nil
}

var _ Store = (*SQL)(nil)
