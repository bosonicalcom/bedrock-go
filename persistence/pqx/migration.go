package pqx

import (
	"context"
	"errors"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
)

// InstanceType represents the type of instance running the migrations.
type InstanceType uint8

// RunMigrationsParams groups the inputs required by [RunMigrations].
// MasterConf must have superuser privileges to create roles and databases.
// RuntimeConf is the application-level credential used to run the migrations themselves.
// FS must contain the migration SQL files at its root.
type RunMigrationsParams struct {
	MasterConf   Config
	RuntimeConf  Config
	FS           fs.FS
	InstanceType InstanceType
}

const (
	// InstanceTypeSelfManaged represents a self-managed instance (e.g., a local PostgreSQL server).
	InstanceTypeSelfManaged InstanceType = iota
	// InstanceTypeRDS represents an RDS instance (e.g., AWS RDS PostgreSQL).
	InstanceTypeRDS
)

// RunMigrations runs the database migrations for the given parameters.
// It first provisions the initial artifacts (e.g., roles, extensions) and then applies the migrations.
func RunMigrations(ctx context.Context, params *RunMigrationsParams) error {
	err := provisionInitialArtifacts(ctx, params)
	if err != nil {
		return err
	}

	db, err := pgx.Connect(ctx, params.RuntimeConf.DSN().String())
	if err != nil {
		return err
	}
	defer db.Close(ctx)

	migrationsFS, err := iofs.New(params.FS, ".")
	if err != nil {
		return err
	}

	runtimeDSN := params.RuntimeConf.DSN()
	runtimeDSN.Scheme = "pgx5"
	migrator, err := migrate.NewWithSourceInstance("iofs", migrationsFS, runtimeDSN.String())
	if err != nil {
		return err
	}
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func provisionInitialArtifacts(ctx context.Context, params *RunMigrationsParams) error {
	db, err := pgx.Connect(ctx, params.MasterConf.DSN().String())
	if err != nil {
		return err
	}
	defer db.Close(ctx)

	// PostgreSQL DDL cannot use bind parameters and has no CREATE ROLE ... IF
	// NOT EXISTS, so build the statements with format(%I,%L) — %I quotes the
	// identifier and %L the password literal — then execute the result. The
	// password is (re)set on every start so it stays in sync with the secret.
	var roleExists bool
	err = db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)", params.RuntimeConf.User).Scan(&roleExists)
	if err != nil {
		return err
	}
	verb := "CREATE"
	if roleExists {
		verb = "ALTER"
	}
	var roleStmt string
	err = db.QueryRow(ctx, "SELECT format('"+verb+" ROLE %I WITH LOGIN PASSWORD %L', $1::text, $2::text)", params.RuntimeConf.User, params.RuntimeConf.Password).Scan(&roleStmt)
	if err != nil {
		return err
	}
	if _, err = db.Exec(ctx, roleStmt); err != nil {
		return err
	}

	var dbExists bool
	err = db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", params.RuntimeConf.Database).Scan(&dbExists)
	if err != nil {
		return err
	}
	if dbExists {
		return nil
	}

	if params.InstanceType == InstanceTypeRDS {
		// PostgreSQL requires the creating role to be a MEMBER of the role it assigns
		// as owner (CREATE DATABASE ... OWNER). A true superuser bypasses this, but
		// the RDS master is rds_superuser, not a real superuser, so the rule applies.
		// Grant membership first so master can SET ROLE to the runtime role.
		var grantStmt string
		err = db.QueryRow(ctx, "SELECT format('GRANT %I TO %I', $1::text, $2::text)", params.RuntimeConf.User, params.MasterConf.User).Scan(&grantStmt)
		if err != nil {
			return err
		}
		if _, err = db.Exec(ctx, grantStmt); err != nil {
			return err
		}
	}

	var dbStmt string
	err = db.QueryRow(ctx, "SELECT format('CREATE DATABASE %I OWNER %I', $1::text, $2::text)", params.RuntimeConf.Database, params.RuntimeConf.User).Scan(&dbStmt)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, dbStmt)
	return err
}
