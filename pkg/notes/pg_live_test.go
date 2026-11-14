package notes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestPGRepoContract runs the Repo contract against real Postgres. It is skipped
// unless NOTES_LIVE_DB=1 and DATABASE_URL_UNPOOLED is set, and it works in a
// throwaway schema that it creates and drops, so real tables are never touched:
//
//	NOTES_LIVE_DB=1 DATABASE_URL_UNPOOLED=postgres://... go test ./pkg/notes -run PG -v
func TestPGRepoContract(t *testing.T) {
	url := os.Getenv("DATABASE_URL_UNPOOLED")
	if os.Getenv("NOTES_LIVE_DB") != "1" || url == "" {
		t.Skip("set NOTES_LIVE_DB=1 and DATABASE_URL_UNPOOLED to run against real Postgres")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	b := make([]byte, 5)
	rand.Read(b)
	schema := "notes_t_" + hex.EncodeToString(b)
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err := admin.Exec(ctx, "create schema "+schema); err != nil {
		t.Fatal(err)
	}
	// deferred after admin.Close, so it runs first, while the connection is still open
	defer func() {
		if _, err := admin.Exec(context.Background(), "drop schema "+schema+" cascade"); err != nil {
			t.Errorf("could not drop the throwaway schema %s: %v", schema, err)
		}
	}()

	pg, err := openPG(ctx, url, schema)
	if err != nil {
		t.Fatal(err)
	}
	defer pg.Close()
	// Never trust the isolation: if the connection isn't in the throwaway schema, stop before
	// anything is created or truncated (once real notes exist, that would wipe them).
	var cur string
	if err := pg.pool.QueryRow(ctx, "select current_schema()").Scan(&cur); err != nil || cur != schema {
		t.Fatalf("connection is in schema %q (err %v), not the throwaway %q: refusing to run", cur, err, schema)
	}
	if err := pg.Migrate(ctx); err != nil {
		t.Fatal("Migrate:", err)
	}
	if err := pg.Migrate(ctx); err != nil {
		t.Fatal("Migrate must be safe to run twice:", err)
	}
	repoContract(t, func(t *testing.T) Repo {
		q := pgx.Identifier{schema}.Sanitize() + "."
		if _, err := pg.pool.Exec(ctx, "truncate "+q+"notes, "+q+"votes, "+q+"reports"); err != nil {
			t.Fatal(err)
		}
		return pg
	})
}
