// `llm-recall gold` — scan the last N days of sessions, ask the LLM for
// the Top-10 most quotable user lines, render as a lipgloss list (or
// markdown with --md).
//
// Pipeline:
//
//  1. Load all sessions from cache, filter by updated_at >= now - N days.
//  2. Truncate each body to 1 KB (UTF-8 safe rune boundary). Join with
//     "--- session <id> ---" separators.
//  3. If total ≥ 100 KB, sample 50 sessions deterministically (sort by
//     time desc, then take first 50) so behaviour is reproducible.
//  4. Redact, estimate, confirm.
//  5. LLM call. Parse JSON; on parse failure, retry once with the
//     stricter system prompt. On second failure, surface the raw text.
//  6. Render: lipgloss bordered list (default) OR plain markdown (--md).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xiao98/llm-recall/internal/adapter"
	"github.com/xiao98/llm-recall/internal/config"
	"github.com/xiao98/llm-recall/internal/index"
	"github.com/xiao98/llm-recall/internal/llm"
	"github.com/xiao98/llm-recall/internal/llm/prompts"
	"github.com/xiao98/llm-recall/internal/promo"
)

const (
	goldPerSessionCap = 1024       // bytes; per-session UTF-8-safe truncation
	goldTotalThresh   = 100 * 1024 // ≥100KB → sample
	goldSampleSize    = 50         // sessions to keep when over threshold
	goldMaxOutputToks = 1024
)

func cmdGold(args []string, cfg *config.Config) {
	fs := flag.NewFlagSet("gold", flag.ExitOnError)
	days := fs.Int("days", 7, "look back N days (default 7)")
	yes := fs.Bool("y", false, "skip cost-confirm prompt")
	mdOut := fs.Bool("md", false, "plain markdown output (no ANSI / borders / footer)")
	noCache := fs.Bool("no-cache", false, "skip cache read; still write")
	flagVendor := fs.String("vendor", "", "anthropic|openai")
	flagModel := fs.String("model", "", "model id (overrides default)")
	flagBaseURL := fs.String("llm-base-url", "", "API base URL (overrides default)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *days < 1 {
		*days = 7
	}

	// Load cache.
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

	cutoff := time.Now().Add(-time.Duration(*days) * 24 * time.Hour)
	var sessions []adapter.Session
	for _, src := range []string{"claude", "codex", "gemini"} {
		rows, err := cache.ListBySource(src)
		if err != nil {
			continue
		}
		for _, s := range rows {
			if s.UpdatedAt.After(cutoff) || s.UpdatedAt.Equal(cutoff) {
				sessions = append(sessions, s)
			}
		}
	}
	if len(sessions) == 0 {
		fmt.Fprintln(os.Stderr, "no sessions in the window; run 'llm-recall ls' to refresh.")
		os.Exit(1)
	}

	// Sort by UpdatedAt desc for deterministic sampling.
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	// Build the joined body, truncating each session to goldPerSessionCap.
	bodies := make([]string, 0, len(sessions))
	totalBytes := 0
	for _, s := range sessions {
		chunk := truncUTF8(s.Body, goldPerSessionCap)
		piece := fmt.Sprintf("--- session %s ---\n%s", shortID(s.ID), chunk)
		bodies = append(bodies, piece)
		totalBytes += len(piece)
	}

	// Auto-sample on overlength to keep cost bounded.
	if totalBytes >= goldTotalThresh && len(sessions) > goldSampleSize {
		fmt.Fprintf(os.Stderr,
			"warning: %d sessions / %d bytes exceeds %d-byte threshold; sampling top %d by recency\n",
			len(sessions), totalBytes, goldTotalThresh, goldSampleSize)
		bodies = bodies[:goldSampleSize]
	}

	joined := strings.Join(bodies, "\n\n")
	cleanBodies, redactCount := llm.Redact(joined)
	if redactCount > 0 {
		fmt.Fprintf(os.Stderr, "redacted %d item(s) before LLM call\n", redactCount)
	}

	// Settings.
	settings, err := llm.Resolve(*flagVendor, *flagModel, *flagBaseURL, cfg)
	if err != nil {
		if errors.Is(err, llm.ErrNoCredentials) {
			fmt.Fprintln(os.Stderr, "error: "+llm.FriendlyNoCredsHint)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	system := prompts.SystemGold
	prompt := strings.ReplaceAll(prompts.PromptGoldTpl, "{bodies}", cleanBodies)

	inputToks := llm.EstimateTokens(system + prompt)
	cost := llm.EstimateCostUSD(settings.Model, inputToks, goldMaxOutputToks)

	if !*yes {
		fmt.Fprintf(os.Stderr,
			"扫描 %d 个会话, 估算 token: %d input / ~%d output, 预估成本: %s USD (%s)\n",
			len(bodies), inputToks, goldMaxOutputToks, llm.FormatCostUSD(cost), settings.Model)
		if !confirmCost(inputToks, goldMaxOutputToks, cost, settings) {
			fmt.Fprintln(os.Stderr, "cancelled.")
			os.Exit(0)
		}
	} else {
		fmt.Fprintf(os.Stderr,
			"扫描 %d 个会话, 估算 token: %d input / ~%d output, 预估成本: %s USD (%s)\n",
			len(bodies), inputToks, goldMaxOutputToks, llm.FormatCostUSD(cost), settings.Model)
	}

	// Cache + call.
	cacheKey := llm.CacheKey(settings.Model, system, prompt)
	var resp llm.Response
	if !*noCache {
		if r, ok := llm.CacheGet(cacheKey); ok {
			resp = r
		}
	}
	if resp.Text == "" {
		fmt.Fprintf(os.Stderr, "调用 %s %s...\n", settings.Vendor, settings.Model)
		client, err := llm.NewClient(settings.Vendor, settings.Key, settings.Model, settings.BaseURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		ctx := context.Background()
		got, err := client.Complete(ctx, llm.Request{System: system, Prompt: prompt, MaxTokens: goldMaxOutputToks})
		if err != nil {
			fmt.Fprintln(os.Stderr, "error: "+llm.FriendlyAPIError(err))
			os.Exit(1)
		}
		resp = got
		_ = llm.CachePut(cacheKey, settings.Model, resp)
	}

	// Parse JSON. On failure, retry once with stricter system prompt
	// (only if not in mock mode and not a cached miss path that already
	// retried).
	entries, err := parseGoldResponse(resp.Text)
	if err != nil {
		// Retry with strict prompt.
		fmt.Fprintln(os.Stderr, "warn: first response did not parse as JSON; retrying with stricter prompt")
		client2, err2 := llm.NewClient(settings.Vendor, settings.Key, settings.Model, settings.BaseURL)
		if err2 != nil {
			fmt.Fprintln(os.Stderr, "error: "+err2.Error())
			os.Exit(1)
		}
		strictReq := llm.Request{
			System:    prompts.SystemGoldStrict,
			Prompt:    prompt,
			MaxTokens: goldMaxOutputToks,
		}
		retry, rerr := client2.Complete(context.Background(), strictReq)
		if rerr != nil {
			fmt.Fprintln(os.Stderr, "error: "+llm.FriendlyAPIError(rerr))
			os.Exit(1)
		}
		entries, err = parseGoldResponse(retry.Text)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: model output not parseable as JSON after retry: %v\nraw: %s\n", err, llm.SafeTrunc(retry.Text, 400))
			os.Exit(1)
		}
	}

	// Render.
	gd := llm.GoldData{
		WindowDays: *days,
		Entries:    entries,
		Footer:     promo.StatsFooter(cfg),
	}
	if *mdOut {
		// --md: plain markdown, no footer / colour.
		gd.Footer = ""
		fmt.Print(llm.RenderGoldMD(gd))
		return
	}
	fmt.Println(llm.RenderGold(gd))
}

// parseGoldResponse extracts the JSON array from the model output. We
// tolerate a leading code-fence (``` or ```json) since some models add
// it despite the system prompt.
func parseGoldResponse(text string) ([]llm.GoldEntry, error) {
	t := strings.TrimSpace(text)
	// Strip code fences if present.
	if strings.HasPrefix(t, "```") {
		// Drop the first line (``` or ```json) and the closing fence.
		if i := strings.Index(t, "\n"); i > 0 {
			t = t[i+1:]
		}
		if j := strings.LastIndex(t, "```"); j > 0 {
			t = t[:j]
		}
		t = strings.TrimSpace(t)
	}
	// Expect an array. If the model wrapped in {entries: [...]}, lift.
	type rawEntry struct {
		Quote   string `json:"quote"`
		Comment string `json:"comment"`
	}
	var arr []rawEntry
	if err := json.Unmarshal([]byte(t), &arr); err != nil {
		// Try wrapped object.
		var wrap struct {
			Entries []rawEntry `json:"entries"`
		}
		if werr := json.Unmarshal([]byte(t), &wrap); werr == nil && len(wrap.Entries) > 0 {
			arr = wrap.Entries
		} else {
			return nil, err
		}
	}
	out := make([]llm.GoldEntry, 0, len(arr))
	for _, e := range arr {
		q := strings.TrimSpace(e.Quote)
		c := strings.TrimSpace(e.Comment)
		if q == "" {
			continue
		}
		out = append(out, llm.GoldEntry{Quote: q, Comment: c})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no usable entries in response")
	}
	return out, nil
}

// truncUTF8 returns s truncated to ≤ maxBytes bytes, snapped to the
// nearest UTF-8 rune boundary at or below maxBytes (never splits a
// multi-byte rune in half).
func truncUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	// Snap down to a valid boundary.
	end := maxBytes
	for end > 0 {
		if utf8.RuneStart(s[end]) {
			break
		}
		end--
	}
	return s[:end]
}
