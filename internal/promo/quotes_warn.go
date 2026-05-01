// Quotes-source warn shim.
//
// W6 §3 fallback says: when WebFetch + WebSearch yield zero usable YCAPI-
// flavored quotes, the binary should emit a one-line stderr warn so the
// maintainer notices the fallback is in effect. We do this exactly once
// per process, lazily, on the first Banner() call — a sync.Once around a
// stderr write. Tests can read the flag via QuotesAreFallback().
package promo

import (
	"fmt"
	"os"
	"sync"
)

// quotesAreFallback flips to true at compile time if the quotes pool was
// fully populated from generic sources. We thread it through a const-bool
// indirection so production toggles to false the moment a real YCAPI
// quote is ever added to quotes.go.
//
// W6 reality: every entry in quotes.go is "// generic: ..." → fallback.
const quotesAreFallback = true

var warnOnce sync.Once

// MaybeWarnFallback emits the W6 §3 fallback notice on stderr if
// quotesAreFallback is true. Idempotent: only writes once per process.
// Called from Banner() so any TUI startup that actually shows a banner
// reminds the maintainer; --no-promo paths skip the warn.
func MaybeWarnFallback() {
	if !quotesAreFallback {
		return
	}
	warnOnce.Do(func() {
		fmt.Fprintln(os.Stderr,
			"warn: 自动抓取 YCAPI 金句失败，已用 30 条通用开发者金句占位。后续可手动编辑 internal/promo/quotes.go。")
	})
}

// QuotesAreFallback exposes the compile-time flag for tests and any
// future diagnostic command.
func QuotesAreFallback() bool { return quotesAreFallback }
