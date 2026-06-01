// Package pqx provides PostgreSQL connection configuration and database migration helpers.
package pqx

import (
	"fmt"
	"net/url"
)

// Config holds the connection parameters for a PostgreSQL instance.
// ModeSSL must be "disable" for plain connections or "verify-full" (recommended)
// for TLS-verified connections. RootCertPathSSL is only used when ModeSSL is not "disable".
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	ModeSSL         string
	RootCertPathSSL string
}

// Insecure returns true if SSL mode is disabled.
func (c Config) Insecure() bool {
	return c.ModeSSL == "disable"
}

// DSN returns a *url.URL with the postgres:// scheme suitable for passing to
// pgx or the golang-migrate driver. SSL query parameters are set automatically
// based on [Config.ModeSSL] and [Config.RootCertPathSSL].
func (c Config) DSN() *url.URL {
	dsn := &url.URL{}
	dsn.Scheme = "postgres"
	dsn.User = url.UserPassword(c.User, c.Password)
	dsn.Host = fmt.Sprintf("%s:%d", c.Host, c.Port)
	dsn.Path = c.Database

	queries := url.Values{}
	if !c.Insecure() {
		queries.Set("sslmode", "verify-full")
		if c.RootCertPathSSL != "" {
			queries.Set("sslrootcert", c.RootCertPathSSL)
		}
	} else {
		queries.Set("sslmode", "disable")
	}
	dsn.RawQuery = queries.Encode()
	return dsn
}
