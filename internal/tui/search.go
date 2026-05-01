package tui

import (
	"database/sql"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/xiao98/llm-recall/internal/search"
)

// latestStamp is shared across all live runSearchCmd closures. Each cmd
// captures its own stamp at schedule time; before doing the expensive
// fuzzy/SQL work it confirms it's still the freshest stamp. Stale cmds
// short-circuit and emit a searchMsg the model will discard. This is the
// "cancel earlier searches when a new keystroke arrives" mechanism — without
// it, every keystroke's cmd runs to completion concurrently, saturating CPU
// for 1+ seconds during fast typing.
var latestStamp uint64

// searchMsg is the result of one Search call. Stamp is the monotonic counter
// the model assigned when it kicked off this query — Update drops messages
// whose Stamp is behind the latest, so a slow search lagging a fast typist
// can't overwrite fresh results.
type searchMsg struct {
	Stamp   uint64
	Query   string
	Source  string
	Results []search.Result
	Err     error
}

// debounceWindow is the keystroke quiet period before a search fires. 50 ms
// matches the W3 task doc and is short enough that users feel the list
// updating "live" without burning a query per keystroke when typing fast.
const debounceWindow = 50 * time.Millisecond

// runSearchCmd schedules a debounced search. The returned tea.Cmd sleeps for
// debounceWindow then runs the SQL+fuzzy pipeline. The model holds the
// monotonic Stamp so Update can compare and discard stale answers.
func runSearchCmd(db *sql.DB, stamp uint64, query, source string) tea.Cmd {
	atomic.StoreUint64(&latestStamp, stamp)
	return func() tea.Msg {
		time.Sleep(debounceWindow)
		// Drop the search if a newer keystroke has fired in the meantime.
		// Without this every keystroke's cmd runs to completion concurrently
		// during fast typing, saturating CPU and producing stale results.
		if atomic.LoadUint64(&latestStamp) != stamp {
			return searchMsg{Stamp: stamp, Query: query, Source: source}
		}
		results, err := search.Search(db, query, source, 200)
		return searchMsg{
			Stamp:   stamp,
			Query:   query,
			Source:  source,
			Results: results,
			Err:     err,
		}
	}
}
