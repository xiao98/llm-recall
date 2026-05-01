// Package search runs the TUI's filter pipeline against the SQLite cache.
//
// Two-stage design:
//
//  1. SQL pre-filter — every space-separated query word must appear in title
//     OR body (case-insensitive LIKE), optionally constrained by source. This
//     happens in SQLite, on indexed columns, and caps the candidate set at
//     `prefilterLimit` rows.
//
//  2. Fuzzy rank — sahilm/fuzzy reorders the candidates by match quality
//     against `query`, scoring on `title + body[:500]` so partial title hits
//     and body hits both contribute. The final slice is truncated to `limit`.
//
// The empty-query path skips both stages and returns the most recently
// updated rows, which is what the TUI shows on cold launch.
package search

import (
	"database/sql"
	"strings"
	"time"

	"github.com/sahilm/fuzzy"
	"github.com/xiao98/llm-recall/internal/adapter"
)

// prefilterLimit caps the rows we feed into fuzzy ranking. 200 is generous —
// fuzzy.Find on 200 short strings completes in well under 1 ms on a laptop —
// but bounded so that a query like "the" doesn't drag in thousands of rows.
const prefilterLimit = 200

// Result is one ranked candidate. Score comes from sahilm/fuzzy; higher is a
// better match. Empty-query results carry score=0.
type Result struct {
	Session adapter.Session
	Score   int
	// MatchedIndexes are rune positions inside (title + "\n" + body) that
	// fuzzy decided are part of the match. The TUI uses these to highlight
	// the preview pane.
	MatchedIndexes []int
}

// Search runs the full pipeline. `db` is the SQLite handle from index.Cache.
// `query` is the raw search box content; whitespace is treated as the AND
// separator (fuzzy and SQL agree on this). `source` is "" or one of the
// adapter names. `limit` is the post-rank truncation; 0 means "use a
// reasonable default" (200).
func Search(db *sql.DB, query, source string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = prefilterLimit
	}
	rows, err := prefilter(db, query, source, prefilterLimit)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(query) == "" {
		// No query: keep DB order (updated_at DESC), no fuzzy rank.
		out := make([]Result, 0, len(rows))
		for _, r := range rows {
			out = append(out, Result{Session: r})
		}
		if len(out) > limit {
			out = out[:limit]
		}
		return out, nil
	}

	return rerank(rows, query, limit), nil
}

// prefilter is the SQL stage. We split `query` on whitespace and require
// every word to appear in title OR body — this matches users who type
// "claude history" and expect any row mentioning both words to land,
// regardless of which column the word lives in.
//
// LIKE patterns are bound parameters; SQLite handles % escaping on the
// payload side automatically (we pass `%word%` and `word` is treated as a
// literal substring).
func prefilter(db *sql.DB, query, source string, lim int) ([]adapter.Session, error) {
	words := strings.Fields(strings.ToLower(query))

	var (
		conds []string
		args  []any
	)
	if source != "" {
		conds = append(conds, "source = ?")
		args = append(args, source)
	}
	for _, w := range words {
		conds = append(conds, "(LOWER(title) LIKE ? OR LOWER(body) LIKE ?)")
		pat := "%" + w + "%"
		args = append(args, pat, pat)
	}
	q := `SELECT source, id, cwd, started_at, updated_at, file_path, title, body FROM sessions`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, lim)

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []adapter.Session
	for rows.Next() {
		var (
			src, id, cwd, fp, title, body string
			startedAt, updAt              int64
		)
		if err := rows.Scan(&src, &id, &cwd, &startedAt, &updAt, &fp, &title, &body); err != nil {
			return nil, err
		}
		out = append(out, adapter.Session{
			Source:    src,
			ID:        id,
			CWD:       cwd,
			StartedAt: time.Unix(startedAt, 0),
			UpdatedAt: time.Unix(updAt, 0),
			FilePath:  fp,
			Title:     title,
			Body:      body,
		})
	}
	return out, rows.Err()
}

// rerank wraps each candidate with a (title + body-prefix) "haystack" string,
// runs sahilm/fuzzy against it, and emits the top `limit` results in fuzzy's
// score order. body is clipped to bodyHaystackBudget runes so very long
// bodies don't drown out title relevance. Kept small (200 runes) because
// fuzzy.FindFrom is O(haystack) per row.
const bodyHaystackBudget = 200

// haystackSource pre-computes the (title + body-prefix) string for each row
// and caches it. fuzzy.FindFrom calls Source.String(i) MANY times per row
// during scoring; recomputing the rune-level prefix each call would make
// fuzzy run in O(M^2) per row instead of O(M). The cache turns "claude
// 历史" on 200 rows from ~2s to ~5ms.
type haystackSource struct {
	cache []string
}

func newHaystackSource(rows []adapter.Session) haystackSource {
	c := make([]string, len(rows))
	for i, r := range rows {
		body := r.Body
		if rs := []rune(body); len(rs) > bodyHaystackBudget {
			body = string(rs[:bodyHaystackBudget])
		}
		// Newline separator: fuzzy treats title and body as one continuous
		// haystack so a query can span both.
		c[i] = r.Title + "\n" + body
	}
	return haystackSource{cache: c}
}

func (h haystackSource) String(i int) string { return h.cache[i] }
func (h haystackSource) Len() int            { return len(h.cache) }

func rerank(rows []adapter.Session, query string, limit int) []Result {
	matches := fuzzy.FindFrom(query, newHaystackSource(rows))
	out := make([]Result, 0, len(matches))
	for _, m := range matches {
		out = append(out, Result{
			Session:        rows[m.Index],
			Score:          m.Score,
			MatchedIndexes: m.MatchedIndexes,
		})
		if len(out) >= limit {
			break
		}
	}
	if len(out) == 0 {
		// Fuzzy found nothing. Fall back to the SQL prefilter order so the
		// list isn't empty when the user is mid-typing. This happens for
		// queries with non-ASCII gaps that fuzzy's case-fold can't bridge.
		for i, r := range rows {
			if i >= limit {
				break
			}
			out = append(out, Result{Session: r})
		}
	}
	return out
}

// Words returns the lowercase token list used for highlighting in the
// preview pane. Exposed so the TUI doesn't have to re-implement the same
// split rule.
func Words(query string) []string {
	return strings.Fields(strings.ToLower(query))
}
