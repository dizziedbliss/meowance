package notes

import (
	"context"
	_ "embed"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// PG is the Postgres Repo (Neon in production).
type PG struct{ pool *pgxpool.Pool }

// OpenPG connects with a Postgres URL. Use Neon's pooled URL (DATABASE_URL) on
// Vercel: the pool here stays small because every warm instance keeps its own.
func OpenPG(ctx context.Context, url string) (*PG, error) { return openPG(ctx, url, "") }

// openPG also lets tests pin a schema (search_path) to work in a throwaway one.
func openPG(ctx context.Context, url, schema string) (*PG, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	// The simple protocol never prepares statements, so it is safe behind Neon's
	// transaction-pooling PgBouncer, and it lets Migrate run a whole file in one call.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.MaxConns, cfg.MaxConnIdleTime = 3, 30*time.Second
	if schema != "" {
		// a startup parameter is not honoured on every endpoint (Neon ignored it), so set it on each new connection
		cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
			_, err := c.Exec(ctx, "set search_path to "+pgx.Identifier{schema}.Sanitize())
			return err
		}
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &PG{pool: pool}, nil
}

func (p *PG) Close() { p.pool.Close() }

// Migrate creates the tables if they are missing; it is safe to run any number of times.
func (p *PG) Migrate(ctx context.Context) error {
	_, err := p.pool.Exec(ctx, schemaSQL)
	return err
}

// CreateWithin takes a per-student advisory lock inside the transaction, so two
// parallel requests from one student are counted one after the other.
func (p *PG) CreateWithin(ctx context.Context, n Note, since time.Time, max int) (bool, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1, 0))`, n.Uploader); err != nil {
		return false, err
	}
	var have int
	if err := tx.QueryRow(ctx, `select count(*) from notes where uploader = $1 and created_at >= $2`, n.Uploader, since).Scan(&have); err != nil {
		return false, err
	}
	if have >= max {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `insert into notes (id, code, dept, sem, title, storage, file_key, mime, size, uploader, status, created_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		n.ID, n.Code, n.Dept, n.Sem, n.Title, n.Storage, n.FileKey, n.MIME, n.Size, n.Uploader, n.Status, n.Created); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (p *PG) Reject(ctx context.Context, id string) error {
	return p.touch(ctx, `update notes set status = 'rejected' where id = $1 and status = 'pending'`, id)
}

func (p *PG) SetLive(ctx context.Context, id, key string, size int64, mime string) error {
	return p.touch(ctx, `update notes set status = 'live', file_key = $2, size = $3, mime = $4 where id = $1 and status = 'pending'`, id, key, size, mime)
}

// touch runs an update that must change exactly one note.
func (p *PG) touch(ctx context.Context, sql string, args ...any) error {
	tag, err := p.pool.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PG) Delete(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `delete from notes where id = $1`, id)
	return err
}

const noteCols = `id, code, dept, sem, title, storage, file_key, mime, size, uploader, status, created_at,
	(select count(*) from votes v where v.note_id = notes.id)`

func scanNote(row pgx.Row) (Note, error) {
	var n Note
	err := row.Scan(&n.ID, &n.Code, &n.Dept, &n.Sem, &n.Title, &n.Storage, &n.FileKey, &n.MIME, &n.Size, &n.Uploader, &n.Status, &n.Created, &n.Votes)
	return n, err
}

func (p *PG) Get(ctx context.Context, id string) (Note, error) {
	n, err := scanNote(p.pool.QueryRow(ctx, `select `+noteCols+` from notes where id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Note{}, ErrNotFound
	}
	return n, err
}

func (p *PG) ListLive(ctx context.Context, code string) ([]Note, error) {
	rows, err := p.pool.Query(ctx, `select `+noteCols+` from notes where code = $1 and status = 'live' order by created_at desc limit 200`, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Note
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (p *PG) Vote(ctx context.Context, id, usn string) (bool, error) {
	tag, err := p.pool.Exec(ctx, `insert into votes (note_id, usn) values ($1, $2) on conflict do nothing`, id, usn)
	return tag.RowsAffected() == 1, err
}

func (p *PG) Report(ctx context.Context, id, usn, reason string) (bool, error) {
	tag, err := p.pool.Exec(ctx, `insert into reports (note_id, usn, reason) values ($1, $2, $3) on conflict do nothing`, id, usn, reason)
	return tag.RowsAffected() == 1, err
}
