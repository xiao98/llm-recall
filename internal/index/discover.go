// Package index walks every registered adapter and aggregates sessions.
//
// The cache layer keys on (file_path, file_mtime, file_size). On a hit we
// rebuild the Session from cached columns without touching disk; on a miss
// we re-parse. Even when --no-cache is set we still WRITE to the cache so
// the next run is fast — TASKS-W2.md §5 explicitly demands this.
//
// W3 introduces the NeedBody option for the TUI preview pane: when set, the
// discover layer ensures every returned Session has its Body field populated.
// On a v2 schema upgrade the existing rows have `body=""`; we treat those as
// half-hits and run ParseFileFull once to backfill.
package index

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/xiao98/llm-recall/internal/adapter"
)

// Adapters is the registry consulted by DiscoverAll. Tests / future code can
// swap it for a smaller list.
var Adapters = []adapter.SessionAdapter{
	adapter.NewClaude(),
	adapter.NewCodex(),
	adapter.NewGemini(),
}

// Options tunes DiscoverAll. Zero value (UseCache=false, Source="",
// NeedBody=false) behaves like W2 ls: scan everything fresh, return all
// adapters, no body extraction.
type Options struct {
	// UseCache controls cache READ. When true and an adapter implements
	// FileLister + FileParser, we skip parsing files whose (mtime, size)
	// matches the DB row. UseCache=false re-parses everything but still
	// writes results back into the cache.
	UseCache bool

	// Source filters to a single adapter by Name(). Empty == all.
	Source string

	// NeedBody asks the discover layer to guarantee that every returned
	// Session has its Body field populated. Cache hits with empty body
	// (e.g. fresh schema-v2 upgrade where the column exists but every row
	// holds "") trigger a one-time ParseFileFull rescan. The TUI sets this;
	// `ls` does not.
	NeedBody bool
}

// DiscoverAll fans out to every registered adapter, collects sessions, and
// returns them sorted by UpdatedAt descending. A single adapter failure is
// logged to stderr but does not abort the others.
func DiscoverAll(ctx context.Context, opt Options) ([]adapter.Session, error) {
	dbPath, err := CacheDBPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: cache path: %v\n", err)
	}
	var cache *Cache
	if dbPath != "" {
		cache, err = OpenCache(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: cache open: %v\n", err)
			cache = nil
		}
	}
	if cache != nil {
		defer cache.Close()
	}

	var all []adapter.Session
	for _, a := range Adapters {
		if opt.Source != "" && a.Name() != opt.Source {
			continue
		}
		select {
		case <-ctx.Done():
			return all, ctx.Err()
		default:
		}
		sessions, err := discoverOne(ctx, a, cache, opt.UseCache, opt.NeedBody)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: adapter %s discover: %v\n", a.Name(), err)
			continue
		}
		all = append(all, sessions...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].UpdatedAt.After(all[j].UpdatedAt)
	})
	return all, nil
}

// discoverOne handles one adapter with optional caching.
//
//   - When the adapter implements FileLister + FileParser we go through the
//     cache path: list files, ask cache for a hit per file, parse on miss,
//     upsert all results in one transaction, sweep stale rows.
//   - Otherwise we fall back to the adapter's own Discover() and return what
//     it gives us (no body, no upsert).
//
// `needBody` makes the cache hit conditional on Body being non-empty: the v2
// migration leaves all existing rows with body="" until one rescan repopulates
// them. Without this guard, the TUI preview pane would be blank forever after
// upgrade.
func discoverOne(ctx context.Context, a adapter.SessionAdapter, cache *Cache, useCacheRead, needBody bool) ([]adapter.Session, error) {
	lister, lok := a.(adapter.FileLister)
	parser, pok := a.(adapter.FileParser)
	bodyParser, bok := a.(adapter.FileBodyParser)
	if cache == nil || !lok || !pok {
		// No cache available or adapter doesn't expose granular hooks: just
		// call Discover() directly.
		return a.Discover(ctx)
	}

	files, err := lister.ListFiles()
	if err != nil {
		return nil, err
	}

	batch, err := cache.BeginUpsert()
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			batch.Rollback()
		}
	}()

	out := make([]adapter.Session, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, f := range files {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}
		seen[f.Path] = struct{}{}

		if useCacheRead {
			if s, fmtime, fsize, hit, err := cache.GetByPath(a.Name(), f.Path); err == nil && hit {
				bodyOK := !needBody || s.Body != ""
				if fmtime == f.Mtime && fsize == f.Size && bodyOK {
					out = append(out, *s)
					continue
				}
			}
		}
		// Miss (or cache-read disabled, or body backfill needed): parse and upsert.
		// Use ParseFileFull when the caller asked for bodies AND the adapter
		// supports the optional FileBodyParser; otherwise fall back to the
		// title-only ParseFile so callers like `ls` stay fast.
		var (
			s    adapter.Session
			body string
		)
		if needBody && bok {
			s, body, err = bodyParser.ParseFileFull(f.Path)
		} else {
			s, err = parser.ParseFile(f.Path)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: %s parse %s: %v\n", a.Name(), f.Path, err)
			continue
		}
		s.Body = body
		if err := batch.Upsert(s, body, f.Mtime, f.Size); err != nil {
			fmt.Fprintf(os.Stderr, "warn: %s upsert %s: %v\n", a.Name(), f.Path, err)
		}
		out = append(out, s)
	}

	if err := batch.Commit(); err != nil {
		return out, fmt.Errorf("commit: %w", err)
	}
	committed = true

	// Stale sweep: anything in the DB for this source whose path no longer
	// exists on disk is dead and must be deleted, otherwise ls shows zombies.
	dbPaths, err := cache.PathsBySource(a.Name())
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: %s sweep list: %v\n", a.Name(), err)
		return out, nil
	}
	var dead []string
	for p := range dbPaths {
		if _, ok := seen[p]; !ok {
			dead = append(dead, p)
		}
	}
	if len(dead) > 0 {
		if err := cache.DeleteByPaths(a.Name(), dead); err != nil {
			fmt.Fprintf(os.Stderr, "warn: %s sweep delete: %v\n", a.Name(), err)
		}
	}
	return out, nil
}
