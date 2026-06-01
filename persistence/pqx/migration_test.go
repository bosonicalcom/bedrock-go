//go:build integration

package pqx_test

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/suite"

	"github.com/bosonicalcom/bedrock-go/persistence/pqx"
	"github.com/bosonicalcom/bedrock-go/persistence/pqx/pqxtest"
)

func migrationFS() fstest.MapFS {
	return fstest.MapFS{
		"1_init.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE IF NOT EXISTS _migration_smoke_test (id SERIAL PRIMARY KEY);"),
		},
		"1_init.down.sql": &fstest.MapFile{
			Data: []byte("DROP TABLE IF EXISTS _migration_smoke_test;"),
		},
	}
}

type MigrationSuite struct {
	suite.Suite
	cfg pqx.Config
}

func (s *MigrationSuite) SetupSuite() {
	s.cfg = pqxtest.NewContainer(s.T())
}

func (s *MigrationSuite) params() *pqx.RunMigrationsParams {
	return &pqx.RunMigrationsParams{
		MasterConf:   s.cfg,
		RuntimeConf:  s.cfg,
		FS:           migrationFS(),
		InstanceType: pqx.InstanceTypeSelfManaged,
	}
}

func (s *MigrationSuite) TestAppliesMigrations() {
	conn := pqxtest.NewConn(s.T(), s.cfg)
	s.Require().NoError(pqx.RunMigrations(context.Background(), s.params()))

	var exists bool
	err := conn.QueryRow(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = '_migration_smoke_test')",
	).Scan(&exists)
	s.Require().NoError(err)
	s.True(exists)
}

func (s *MigrationSuite) TestIdempotent() {
	s.Require().NoError(pqx.RunMigrations(context.Background(), s.params()))
	s.NoError(pqx.RunMigrations(context.Background(), s.params()))
}

func TestMigrationSuite(t *testing.T) {
	suite.Run(t, new(MigrationSuite))
}
