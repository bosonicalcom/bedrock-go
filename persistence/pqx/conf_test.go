package pqx_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bosonicalcom/bedrock-go/persistence/pqx"
)

func TestConfig_Insecure(t *testing.T) {
	tests := []struct {
		name    string
		modeSSL string
		want    bool
	}{
		{name: "disable", modeSSL: "disable", want: true},
		{name: "verify-full", modeSSL: "verify-full", want: false},
		{name: "empty", modeSSL: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pqx.Config{ModeSSL: tt.modeSSL}.Insecure())
		})
	}
}

func TestConfig_DSN(t *testing.T) {
	base := pqx.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "alice",
		Password: "secret",
		Database: "mydb",
	}

	tests := []struct {
		name         string
		cfg          pqx.Config
		wantSSLMode  string
		wantCertPath string
	}{
		{
			name:        "ssl disabled",
			cfg:         func() pqx.Config { c := base; c.ModeSSL = "disable"; return c }(),
			wantSSLMode: "disable",
		},
		{
			name:        "ssl enabled no cert",
			cfg:         func() pqx.Config { c := base; c.ModeSSL = "verify-full"; return c }(),
			wantSSLMode: "verify-full",
		},
		{
			name:         "ssl enabled with cert",
			cfg:          func() pqx.Config { c := base; c.ModeSSL = "verify-full"; c.RootCertPathSSL = "/certs/ca.pem"; return c }(),
			wantSSLMode:  "verify-full",
			wantCertPath: "/certs/ca.pem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := tt.cfg.DSN()

			assert.Equal(t, "postgres", u.Scheme)
			assert.Equal(t, "localhost:5432", u.Host)
			assert.Equal(t, "mydb", u.Path)

			pw, _ := u.User.Password()
			assert.Equal(t, "alice", u.User.Username())
			assert.Equal(t, "secret", pw)

			q := u.Query()
			assert.Equal(t, tt.wantSSLMode, q.Get("sslmode"))
			assert.Equal(t, tt.wantCertPath, q.Get("sslrootcert"))
		})
	}
}
