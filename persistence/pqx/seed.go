package pqx

import (
	"context"
	"fmt"
	"io"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Seed executes all SQL seed scripts from [fsys] inside a transaction.
func Seed(ctx context.Context, connPool *pgxpool.Pool, fsys fs.FS) error {
	batch := &pgx.Batch{
		QueuedQueries: make([]*pgx.QueuedQuery, 0),
	}
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("bedrock.pqx: cannot walk seed file system: %v", err)
		}

		if d.IsDir() {
			return nil
		}

		file, walkErr := fsys.Open(d.Name())
		if walkErr != nil {
			return fmt.Errorf("bedrock.pqx: cannot open seed file: %w", walkErr)
		}
		defer file.Close()

		content, walkErr := io.ReadAll(file)
		if walkErr != nil {
			return fmt.Errorf("bedrock.pqx: cannot read sql file: %w", walkErr)
		}

		batch.Queue(string(content))
		return nil
	})
	if err != nil {
		return err
	}

	result := connPool.SendBatch(ctx, batch)
	if err = result.Close(); err != nil {
		return fmt.Errorf("bedrock.pqx: cannot execute sql statements: %w", err)
	}
	return nil
}
