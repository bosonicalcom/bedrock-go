// Package pqxtest provides reusable PostgreSQL test helpers backed by testcontainers.
// Import this package in tests that need a real PostgreSQL instance.
package pqxtest

import (
	"context"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/bosonicalcom/bedrock-go/persistence/pqx"
)

const (
	defaultImage    = "postgres:18-alpine"
	defaultUser     = "test"
	defaultPassword = "test"
	defaultDatabase = "testdb"
)

type containerOptions struct {
	image    string
	user     string
	password string
	database string
}

// ContainerOpt customises the PostgreSQL container started by [NewContainer].
type ContainerOpt func(*containerOptions)

// WithImage sets the Docker image used for the container (e.g. "postgres:16-alpine").
func WithImage(image string) ContainerOpt {
	return func(o *containerOptions) { o.image = image }
}

// WithUser sets the PostgreSQL superuser name.
func WithUser(user string) ContainerOpt {
	return func(o *containerOptions) { o.user = user }
}

// WithPassword sets the PostgreSQL superuser password.
func WithPassword(password string) ContainerOpt {
	return func(o *containerOptions) { o.password = password }
}

// WithDatabase sets the default database name.
func WithDatabase(database string) ContainerOpt {
	return func(o *containerOptions) { o.database = database }
}

// NewContainer starts a PostgreSQL container, registers cleanup via tb.Cleanup,
// and returns a Config pointing at the running instance.
func NewContainer(tb testing.TB, opts ...ContainerOpt) pqx.Config {
	tb.Helper()

	o := &containerOptions{
		image:    defaultImage,
		user:     defaultUser,
		password: defaultPassword,
		database: defaultDatabase,
	}
	for _, opt := range opts {
		opt(o)
	}

	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, o.image,
		tcpostgres.WithDatabase(o.database),
		tcpostgres.WithUsername(o.user),
		tcpostgres.WithPassword(o.password),
		// Postgres prints the readiness line twice: once after initdb and once
		// after the server restarts and is truly accepting connections.
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		tb.Fatalf("pqxtest: start postgres container: %v", err)
	}
	tb.Cleanup(func() {
		if err := ctr.Terminate(ctx); err != nil {
			tb.Logf("pqxtest: terminate container: %v", err)
		}
	})

	host, err := ctr.Host(ctx)
	if err != nil {
		tb.Fatalf("pqxtest: get container host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		tb.Fatalf("pqxtest: get mapped port: %v", err)
	}
	portNum, err := strconv.Atoi(port.Port())
	if err != nil {
		tb.Fatalf("pqxtest: parse mapped port: %v", err)
	}

	return pqx.Config{
		Host:     host,
		Port:     portNum,
		User:     o.user,
		Password: o.password,
		Database: o.database,
		ModeSSL:  "disable",
	}
}

// NewConn opens a *pgx.Conn from cfg and registers cleanup via tb.Cleanup.
func NewConn(tb testing.TB, cfg pqx.Config) *pgx.Conn {
	tb.Helper()
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, cfg.DSN().String())
	if err != nil {
		tb.Fatalf("pqxtest: connect to postgres: %v", err)
	}
	tb.Cleanup(func() {
		if err := conn.Close(ctx); err != nil {
			tb.Logf("pqxtest: close connection: %v", err)
		}
	})
	return conn
}
