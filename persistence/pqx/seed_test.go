//go:build integration

package pqx_test

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bosonicalcom/bedrock-go/persistence/pqx"
	"github.com/bosonicalcom/bedrock-go/persistence/pqx/pqxtest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func seedFS() fstest.MapFS {
	return fstest.MapFS{
		"1_foo.sql": &fstest.MapFile{
			Data: []byte("INSERT INTO samples(some_field) VALUES ('foo');"),
		},
		"2_bar.sql": &fstest.MapFile{
			Data: []byte("INSERT INTO samples(some_field) VALUES ('bar');"),
		},
	}
}

func TestSeed(t *testing.T) {
	// arrange
	conf := pqxtest.NewContainer(t)

	connPool, err := pgxpool.New(t.Context(), conf.DSN().String())
	if err != nil {
		t.Fatalf("cannot start postgres pool: %v", err)
	}
	defer connPool.Close()

	migrationStmt := `CREATE TABLE samples(id SERIAL PRIMARY KEY, some_field TEXT DEFAULT NULL)`
	if _, err = connPool.Exec(t.Context(), migrationStmt); err != nil {
		t.Fatalf("cannot exec migrations: %v", err)
	}

	// act
	if err = pqx.Seed(t.Context(), connPool, seedFS()); err != nil {
		t.Fatalf("seeding failed: %v", err)
	}

	// assert
	rows, err := connPool.Query(t.Context(), "SELECT some_field FROM samples ORDER BY some_field DESC")
	if err != nil {
		t.Fatalf("cannot retrieve seeded rows: %v", err)
	}
	defer rows.Close()

	if err = rows.Err(); err != nil {
		t.Fatalf("cannot iterate over seeded rows: %v", err)
	}

	exp := []string{
		"foo",
		"bar",
	}
	out := make([]string, 0, 2)
	for rows.Next() {
		var str string
		if err = rows.Scan(&str); err != nil {
			t.Fatalf("cannot scan seeded row: %v", err)
		}
		out = append(out, str)
	}

	compared := slices.Compare(out, exp)
	if compared != 0 { // not equal
		t.Fatalf("seeded rows do not match, exp [%s], got [%s]", strings.Join(exp, ","), strings.Join(out, ","))
	}
}
