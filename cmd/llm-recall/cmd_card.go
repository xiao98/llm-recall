// `llm-recall card <session-id>` — generate a one-shot lipgloss card
// describing what the user was doing in that session.
//
// Pipeline:
//
//  1. Resolve session by id-prefix from the SQLite cache (W2). Empty
//     match → friendly hint.
//  2. Redact body (PII regex set in internal/llm/redact.go).
//  3. Estimate input tokens + cost. Confirm prompt unless `-y`.
//  4. Cache lookup (sha256 of model+system+prompt). Hit → render. Miss
//     → call LLM → write cache → render.
//  5. Compose CardData and feed RenderCard.
//
// Failure modes route to FriendlyAPIError; we never panic.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/xiao98/llm-recall/internal/adapter"
	"github.com/xiao98/llm-recall/internal/config"
	"github.com/xiao98/llm-recall/internal/index"
	"github.com/xiao98/llm-recall/internal/llm"
	"github.com/xiao98/llm-recall/internal/llm/prompts"
	"github.com/xiao98/llm-recall/internal/promo"
)

// cardMaxOutputToks: gold gives the model room for 10 quotes; card
// only needs ~50 chars. We still pad to 256 for safety in case the
// model adds quote chars or moralises.
const cardMaxOutputToks = 256

func cmdCard(args []string, cfg *config.Config) {
	fs := flag.NewFlagSet("card", flag.ExitOnError)
	yes := fs.Bool("y", false, "skip cost-confirm prompt")
	noCache := fs.Bool("no-cache", false, "skip cache read; still write")
	flagVendor := fs.String("vendor", "", "anthropic|openai (overrides env auto-detect)")
	flagModel := fs.String("model", "", "model id (overrides default)")
	flagBaseURL := fs.String("llm-base-url", "", "API base URL (overrides default)")
	// Card takes the session id as the first positional, but Go's
	// flag package stops at the first non-flag arg. Pre-extract the
	// id (the first arg that doesn't start with '-') so flags can
	// appear in any position relative to the id.
	sid, rest := extractFirstPositional(args)
	if sid == "" {
		fmt.Fprintln(os.Stderr, "usage: llm-recall card <session-id>")
		os.Exit(2)
	}
	if err := fs.Parse(rest); err != nil {
		os.Exit(2)
	}

	// Load + locate session in the cache. We don't re-discover here —
	// `ls` is the canonical "refresh" trigger; if the user just made a
	// session, they should `ls` once first.
	dbPath, err := index.CacheDBPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cache path: %v\n", err)
		os.Exit(1)
	}
	cache, err := index.OpenCache(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open cache: %v\n", err)
		os.Exit(1)
	}
	defer cache.Close()

	sess, err := lookupSession(cache, sid)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// Settings resolution.
	settings, err := llm.Resolve(*flagVendor, *flagModel, *flagBaseURL, cfg)
	if err != nil {
		// W9 friendly hint when no credentials are configured.
		// detect.go returns ErrNoCredentials specifically for this
		// case; everything else surfaces as-is.
		if errors.Is(err, llm.ErrNoCredentials) {
			fmt.Fprintln(os.Stderr, "error: "+llm.FriendlyNoCredsHint)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Build prompt: redact body first.
	body := sess.Body
	if body == "" {
		body = sess.Title // fall back so we don't send an empty prompt
	}
	body = llm.SafeTrunc(body, 8000) // cap for cost predictability
	cleanBody, redactCount := llm.Redact(body)
	if redactCount > 0 {
		fmt.Fprintf(os.Stderr, "redacted %d item(s) before LLM call\n", redactCount)
	}

	system := prompts.SystemCard
	prompt := strings.ReplaceAll(prompts.PromptCardTpl, "{body}", cleanBody)

	inputToks := llm.EstimateTokens(system + prompt)
	cost := llm.EstimateCostUSD(settings.Model, inputToks, cardMaxOutputToks)

	if !*yes {
		if !confirmCost(inputToks, cardMaxOutputToks, cost, settings) {
			fmt.Fprintln(os.Stderr, "cancelled.")
			os.Exit(0)
		}
	}

	// Cache lookup.
	cacheKey := llm.CacheKey(settings.Model, system, prompt)
	var resp llm.Response
	if !*noCache {
		if r, ok := llm.CacheGet(cacheKey); ok {
			resp = r
		}
	}
	if resp.Text == "" {
		client, err := llm.NewClient(settings.Vendor, settings.Key, settings.Model, settings.BaseURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		req := llm.Request{System: system, Prompt: prompt, MaxTokens: cardMaxOutputToks}
		ctx := context.Background()
		got, err := client.Complete(ctx, req)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error: "+llm.FriendlyAPIError(err))
			os.Exit(1)
		}
		resp = got
		_ = llm.CachePut(cacheKey, settings.Model, resp)
	}

	// Compose render data.
	first := firstUserSnippet(sess.Body, 200)
	if first == "" {
		first = sess.Title
	}
	first = llm.SafeTrunc(first, 200)

	data := llm.CardData{
		SessionID8: shortID(sess.ID),
		Source:     sess.Source,
		When:       sess.UpdatedAt.Local().Format("2006-01-02 15:04"),
		FirstUser:  first,
		Action:     strings.TrimSpace(resp.Text),
		CWD:        prettyCWD(sess.CWD),
		Footer:     promo.StatsFooter(cfg),
	}
	fmt.Println(llm.RenderCard(data))
}

// lookupSession finds a session by exact id or id-prefix. Searches all
// sources. Ambiguous prefix → error listing matches.
func lookupSession(cache *index.Cache, idOrPrefix string) (*adapter.Session, error) {
	var hits []adapter.Session
	for _, src := range []string{"claude", "codex", "gemini"} {
		rows, err := cache.ListBySource(src)
		if err != nil {
			continue
		}
		for _, s := range rows {
			if s.ID == idOrPrefix || strings.HasPrefix(s.ID, idOrPrefix) {
				hits = append(hits, s)
			}
		}
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("session %s not found in cache; run 'llm-recall ls' to refresh", idOrPrefix)
	}
	if len(hits) > 1 {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("ambiguous prefix %s; matches:\n", idOrPrefix))
		for _, s := range hits {
			b.WriteString(fmt.Sprintf("  %s  %s\n", s.ID, s.Source))
		}
		return nil, fmt.Errorf("%s", b.String())
	}
	s := hits[0]
	return &s, nil
}

// firstUserSnippet pulls the first user-message-looking chunk from the
// concatenated body. The body is already user-only (adapters filter
// system reminders), so we just take the first line that isn't blank.
func firstUserSnippet(body string, max int) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	// Body fields are joined with a separator like "\n\n" or "---";
	// keep the first chunk before that.
	for _, sep := range []string{"\n---\n", "\n\n---", "\n\n"} {
		if i := strings.Index(body, sep); i > 0 {
			body = body[:i]
			break
		}
	}
	body = strings.TrimSpace(body)
	r := []rune(body)
	if len(r) > max {
		body = string(r[:max-1]) + "…"
	}
	// Collapse internal newlines to spaces so the card stays one block.
	body = strings.ReplaceAll(body, "\n", " ")
	body = strings.ReplaceAll(body, "  ", " ")
	return body
}

// prettyCWD shortens an absolute path for the card. Replaces $HOME with ~.
func prettyCWD(cwd string) string {
	if cwd == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" && strings.HasPrefix(cwd, home) {
		return "~" + cwd[len(home):]
	}
	return cwd
}

// extractFirstPositional returns (firstNonFlag, rest) where rest is
// the input slice minus that one element. Used so `card -y <id>` and
// `card <id> -y` both work — flag.Parse() otherwise stops at the id.
func extractFirstPositional(args []string) (string, []string) {
	for i, a := range args {
		if a == "" || a[0] == '-' {
			continue
		}
		// Found it. Splice it out of `rest`.
		rest := make([]string, 0, len(args)-1)
		rest = append(rest, args[:i]...)
		rest = append(rest, args[i+1:]...)
		return a, rest
	}
	return "", args
}

// confirmCost prints the cost banner and reads a single line from stdin.
// Empty / "y" / "Y" → continue; anything else → abort.
func confirmCost(inputToks, maxOut int, cost float64, settings llm.ResolvedSettings) bool {
	fmt.Fprintf(os.Stderr,
		"call %s %s (~%d input toks, ≤%d output toks); estimated cost: %s USD\n",
		settings.Vendor, settings.Model, inputToks, maxOut, llm.FormatCostUSD(cost))
	fmt.Fprint(os.Stderr, "Continue? [y/N]: ")
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	return line == "y" || line == "Y"
}
