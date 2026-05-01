// Package index — SQLite-backed incremental cache for adapter sessions.
//
// Why a cache: Discover() across three vendors on a hot machine touches a few
// hundred jsonl files; the parse cost dominates `ls` latency. We key on
// (file_path, file_mtime, file_size) — when none of those change, we return
// the cached row without opening the file.
//
// Why modernc.org/sqlite: pure-Go, no cgo, ships in `go install` builds. The
// classic mattn/go-sqlite3 would force every user to have a C toolchain —
// hard rule against that in TASKS-W2.md.
package index

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xiao98/llm-recall/internal/adapter"

	_ "modernc.org/sqlite"
)

// Cache wraps the *sql.DB — exported only so callers can Close() it.
type Cache struct {
	db *sql.DB
}

// OpenCache opens (and creates if needed) the cache file at `path`. It
// MkdirAll's the parent directory and applies the WAL pragmas; callers don't
// have to do any of that.
func OpenCache(path string) (*Cache, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("cache dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Pragmas: WAL for concurrent reads, NORMAL sync trades a tiny crash-safety
	// window for a 10x write speedup, busy_timeout avoids spurious SQLITE_BUSY
	// when two `llm-recall` invocations race.
	for _, pragma := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA synchronous=NORMAL`,
		`PRAGMA busy_timeout=5000`,
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %s: %w", pragma, err)
		}
	}
	// W2 schema: keyed on (source, id), with a path index for stale-sweep
	// joins. No FTS5 yet — search lands in W3.
	const schema = `
CREATE TABLE IF NOT EXISTS sessions (
    source      TEXT NOT NULL,
    id          TEXT NOT NULL,
    cwd         TEXT NOT NULL DEFAULT '',
    started_at  INTEGER NOT NULL DEFAULT 0,
    updated_at  INTEGER NOT NULL DEFAULT 0,
    file_path   TEXT NOT NULL,
    file_mtime  INTEGER NOT NULL,
    file_size   INTEGER NOT NULL DEFAULT 0,
    title       TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (source, id)
);
CREATE INDEX IF NOT EXISTS idx_updated ON sessions(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_path ON sessions(file_path);
`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	return &Cache{db: db}, nil
}

// Close releases the underlying connection pool.
func (c *Cache) Close() error { return c.db.Close() }

// Get returns the cached session keyed by (source, file_path), plus
// (file_mtime, file_size) so the caller can match them against fs.Stat.
// Returns (nil, false, nil) on miss; errors are reserved for real DB problems.
func (c *Cache) GetByPath(source, fpath string) (*adapter.Session, int64, int64, bool, error) {
	const q = `SELECT id, cwd, started_at, updated_at, file_path, file_mtime, file_size, title
                 FROM sessions WHERE source = ? AND file_path = ? LIMIT 1`
	row := c.db.QueryRow(q, source, fpath)
	var (
		id, cwd, fp, title string
		startedAt, updAt   int64
		fmtime, fsize      int64
	)
	if err := row.Scan(&id, &cwd, &startedAt, &updAt, &fp, &fmtime, &fsize, &title); err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, 0, false, nil
		}
		return nil, 0, 0, false, err
	}
	s := &adapter.Session{
		Source:    source,
		ID:        id,
		CWD:       cwd,
		StartedAt: time.Unix(startedAt, 0),
		UpdatedAt: time.Unix(updAt, 0),
		FilePath:  fp,
		Title:     title,
	}
	return s, fmtime, fsize, true, nil
}

// Upsert writes or refreshes one session row.
func (c *Cache) Upsert(s adapter.Session, fmtime, fsize int64) error {
	const q = `
INSERT INTO sessions (source, id, cwd, started_at, updated_at, file_path, file_mtime, file_size, title)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(source, id) DO UPDATE SET
    cwd        = excluded.cwd,
    started_at = excluded.started_at,
    updated_at = excluded.updated_at,
    file_path  = excluded.file_path,
    file_mtime = excluded.file_mtime,
    file_size  = excluded.file_size,
    title      = excluded.title`
	_, err := c.db.Exec(q,
		s.Source, s.ID, s.CWD,
		s.StartedAt.Unix(), s.UpdatedAt.Unix(),
		s.FilePath, fmtime, fsize, s.Title,
	)
	return err
}

// UpsertTx wraps a batch of upserts in a single transaction. Returns a
// helper that the caller invokes per row, plus a commit function. Commit MUST
// be called; on error or panic, the tx is rolled back.
type UpsertBatch struct {
	tx     *sql.Tx
	stmt   *sql.Stmt
	closed bool
}

func (c *Cache) BeginUpsert() (*UpsertBatch, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return nil, err
	}
	const q = `
INSERT INTO sessions (source, id, cwd, started_at, updated_at, file_path, file_mtime, file_size, title)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(source, id) DO UPDATE SET
    cwd        = excluded.cwd,
    started_at = excluded.started_at,
    updated_at = excluded.updated_at,
    file_path  = excluded.file_path,
    file_mtime = excluded.file_mtime,
    file_size  = excluded.file_size,
    title      = excluded.title`
	stmt, err := tx.Prepare(q)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return &UpsertBatch{tx: tx, stmt: stmt}, nil
}

func (b *UpsertBatch) Upsert(s adapter.Session, fmtime, fsize int64) error {
	if b.closed {
		return fmt.Errorf("upsert on closed batch")
	}
	_, err := b.stmt.Exec(
		s.Source, s.ID, s.CWD,
		s.StartedAt.Unix(), s.UpdatedAt.Unix(),
		s.FilePath, fmtime, fsize, s.Title,
	)
	return err
}

func (b *UpsertBatch) Commit() error {
	if b.closed {
		return nil
	}
	b.closed = true
	b.stmt.Close()
	return b.tx.Commit()
}

func (b *UpsertBatch) Rollback() error {
	if b.closed {
		return nil
	}
	b.closed = true
	b.stmt.Close()
	return b.tx.Rollback()
}

// DeleteByPaths removes rows whose file_path is in the given list AND whose
// source matches. Used by stale-sweep to drop sessions whose backing file
// vanished from disk. No-op for an empty list.
func (c *Cache) DeleteByPaths(source string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	// Chunk to keep the SQL parameter list under SQLite's default 999 limit.
	const chunkSize = 500
	for start := 0; start < len(paths); start += chunkSize {
		end := start + chunkSize
		if end > len(paths) {
			end = len(paths)
		}
		chunk := paths[start:end]
		placeholders := make([]string, len(chunk))
		args := make([]any, 0, len(chunk)+1)
		args = append(args, source)
		for i, p := range chunk {
			placeholders[i] = "?"
			args = append(args, p)
		}
		q := "DELETE FROM sessions WHERE source = ? AND file_path IN (" + strings.Join(placeholders, ",") + ")"
		if _, err := c.db.Exec(q, args...); err != nil {
			return err
		}
	}
	return nil
}

// ListBySource returns every cached session for one adapter, ordered by
// updated_at desc. Used when UseCache=true and the per-file existence check
// has already confirmed nothing parsed.
func (c *Cache) ListBySource(source string) ([]adapter.Session, error) {
	const q = `SELECT id, cwd, started_at, updated_at, file_path, title
                 FROM sessions WHERE source = ? ORDER BY updated_at DESC`
	rows, err := c.db.Query(q, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adapter.Session
	for rows.Next() {
		var (
			id, cwd, fp, title string
			startedAt, updAt   int64
		)
		if err := rows.Scan(&id, &cwd, &startedAt, &updAt, &fp, &title); err != nil {
			return nil, err
		}
		out = append(out, adapter.Session{
			Source:    source,
			ID:        id,
			CWD:       cwd,
			StartedAt: time.Unix(startedAt, 0),
			UpdatedAt: time.Unix(updAt, 0),
			FilePath:  fp,
			Title:     title,
		})
	}
	return out, rows.Err()
}

// PathsBySource returns the set of file paths known for one adapter. Stale
// sweep diffs this against what's currently on disk.
func (c *Cache) PathsBySource(source string) (map[string]struct{}, error) {
	const q = `SELECT file_path FROM sessions WHERE source = ?`
	rows, err := c.db.Query(q, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out[p] = struct{}{}
	}
	return out, rows.Err()
}
