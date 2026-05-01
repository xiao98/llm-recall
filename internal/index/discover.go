// Package index walks every registered adapter and aggregates sessions.
// W1 has only Claude; the registration shape is what avoids a refactor in W2
// when Codex/Gemini land.
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
}

// DiscoverAll fans out to every registered adapter, collects sessions, and
// returns them sorted by UpdatedAt descending. A single adapter failure is
// logged to stderr but does not abort the others.
func DiscoverAll(ctx context.Context) ([]adapter.Session, error) {
	var all []adapter.Session
	for _, a := range Adapters {
		sessions, err := a.Discover(ctx)
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
