// `llm-recall login` — interactive LLM provider configuration (W9).
//
// Replaces the W7 "stick a key in your shell rc" workflow. The key is
// stored either in `~/.config/llm-recall/credentials.toml` (chmod 600,
// default) or the OS keyring (`--use-keyring`). The key NEVER comes
// through a CLI flag — only an interactive hidden prompt or stdin pipe.
//
// Surface:
//
//	llm-recall login                                          # full interactive
//	llm-recall login --vendor openai --base-url https://X     # non-interactive scaffold
//	echo "$KEY" | llm-recall login --vendor openai --pipe-key # CI-friendly
//	llm-recall login --use-keyring                            # store in OS keyring
//
// Scripted mode rules:
//   - --vendor / --base-url / --model are taken verbatim
//   - --pipe-key reads stdin → strips trailing newline → uses as APIKey
//   - without --pipe-key but with stdin pipe attached AND --vendor set,
//     we still pull the key from stdin (matches user habit:
//     `echo $KEY | llm-recall login --vendor openai`)
//   - missing --vendor in non-tty mode → error (we will not silently
//     pick a default and write the wrong section)
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/xiao98/llm-recall/internal/credentials"
	"github.com/xiao98/llm-recall/internal/llm"
)

// loginDefaults pairs the common base URLs / model ids per vendor so
// the prompt can show a default the user can just press Enter through.
type loginDefaults struct {
	BaseURL string
	Model   string
}

func defaultsFor(vendor string) loginDefaults {
	switch vendor {
	case "anthropic":
		return loginDefaults{
			BaseURL: llm.DefaultBaseURL(llm.Anthropic),
			Model:   llm.DefaultModel(llm.Anthropic),
		}
	case "openai":
		return loginDefaults{
			BaseURL: llm.DefaultBaseURL(llm.OpenAI) + "/v1",
			Model:   llm.DefaultModel(llm.OpenAI),
		}
	}
	return loginDefaults{}
}

func cmdLogin(args []string) {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	flagVendor := fs.String("vendor", "", "anthropic|openai (skip interactive vendor picker)")
	flagBaseURL := fs.String("base-url", "", "API base URL (skip prompt)")
	flagModel := fs.String("model", "", "model id (skip prompt)")
	flagUseKeyring := fs.Bool("use-keyring", false, "store key in OS keyring instead of credentials.toml")
	flagPipeKey := fs.Bool("pipe-key", false, "read key from stdin (one line, trailing newline trimmed)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	stdinIsPipe := !isStdinTTY()
	stdoutIsTTY := isStdoutTTY()

	// Decide vendor.
	vendor := strings.ToLower(strings.TrimSpace(*flagVendor))
	switch vendor {
	case "", "anthropic", "openai":
	default:
		fmt.Fprintf(os.Stderr, "error: --vendor must be anthropic|openai, got %q\n", vendor)
		os.Exit(2)
	}
	if vendor == "" {
		if !stdoutIsTTY {
			fmt.Fprintln(os.Stderr, "error: --vendor required when stdout is not a TTY (interactive prompt would never appear)")
			os.Exit(2)
		}
		v, err := promptVendor(os.Stdin, os.Stderr)
		if err != nil {
			if errors.Is(err, errSkipLogin) {
				fmt.Fprintln(os.Stderr, "skipped — gold / card disabled until you run `llm-recall login` again.")
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		vendor = v
	}

	defaults := defaultsFor(vendor)

	// Decide BaseURL / Model. Flag wins, then prompt-with-default in
	// interactive mode, otherwise fall back to vendor default.
	baseURL := strings.TrimSpace(*flagBaseURL)
	model := strings.TrimSpace(*flagModel)
	if stdoutIsTTY && !stdinIsPipe {
		if baseURL == "" {
			baseURL = promptWithDefault(os.Stdin, os.Stderr,
				"Base URL", defaults.BaseURL)
		}
		if model == "" {
			model = promptWithDefault(os.Stdin, os.Stderr,
				"Model", defaults.Model)
		}
	} else {
		if baseURL == "" {
			baseURL = defaults.BaseURL
		}
		if model == "" {
			model = defaults.Model
		}
	}

	// Decide storage.
	useKeyring := *flagUseKeyring
	if !useKeyring && stdoutIsTTY && !stdinIsPipe {
		useKeyring = promptStorage(os.Stdin, os.Stderr)
	}

	// Read API key.
	key, err := readAPIKey(stdinIsPipe, *flagPipeKey, stdoutIsTTY)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if strings.TrimSpace(key) == "" {
		fmt.Fprintln(os.Stderr, "error: empty API key; aborting (nothing written)")
		os.Exit(1)
	}

	cred := credentials.Cred{
		Vendor:     vendor,
		APIKey:     key,
		BaseURL:    baseURL,
		Model:      model,
		UseKeyring: useKeyring,
	}

	if useKeyring {
		if err := credentials.SaveToKeyring(cred); err != nil {
			if errors.Is(err, credentials.ErrKeyringUnavailable) {
				fmt.Fprintf(os.Stderr,
					"warn: system keyring unavailable (%v); falling back to credentials.toml\n", err)
				cred.UseKeyring = false
			} else {
				fmt.Fprintf(os.Stderr, "error: keyring save: %v\n", err)
				os.Exit(1)
			}
		}
	}
	if err := credentials.Save(cred); err != nil {
		fmt.Fprintf(os.Stderr, "error: write credentials.toml: %v\n", err)
		os.Exit(1)
	}

	storage := credentials.Path()
	if cred.UseKeyring {
		fmt.Fprintf(os.Stderr,
			"saved (vendor=%s, base_url=%s, model=%s)\n  - api key: OS keyring (service \"llm-recall\", key %q)\n  - marker:  %s\nTest with: llm-recall card <session-id>\n",
			vendor, baseURL, model, vendor, storage)
	} else {
		fmt.Fprintf(os.Stderr,
			"saved (vendor=%s, base_url=%s, model=%s)\n  - api key: %s (chmod 600)\nTest with: llm-recall card <session-id>\n",
			vendor, baseURL, model, storage)
	}
}

// errSkipLogin signals that the user picked option [3] in the
// interactive vendor picker. Treated as a clean exit, not an error.
var errSkipLogin = errors.New("user skipped login")

// promptVendor presents the W9 §2.3 picker and returns the canonical
// vendor name (or errSkipLogin for option 3).
func promptVendor(stdin io.Reader, stderr io.Writer) (string, error) {
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "llm-recall LLM setup")
	fmt.Fprintln(stderr, "────────────────────────────────────────")
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "Choose a provider:")
	fmt.Fprintln(stderr, "  [1] Anthropic (Claude)")
	fmt.Fprintln(stderr, "  [2] OpenAI / OpenAI-compatible (含 YC TECH 中转 https://dash.youchun.tech/v1)")
	fmt.Fprintln(stderr, "  [3] Skip — gold / card disabled")
	fmt.Fprintln(stderr, "")

	r := bufio.NewReader(stdin)
	for {
		fmt.Fprint(stderr, "Choose [1/2/3]: ")
		line, err := r.ReadString('\n')
		if err != nil && line == "" {
			return "", err
		}
		switch strings.TrimSpace(line) {
		case "1":
			return "anthropic", nil
		case "2":
			return "openai", nil
		case "3":
			return "", errSkipLogin
		default:
			fmt.Fprintln(stderr, "  (please enter 1, 2, or 3)")
		}
	}
}

// promptWithDefault asks for one line, returns the supplied default
// when the user just hits Enter.
func promptWithDefault(stdin io.Reader, stderr io.Writer, label, def string) string {
	r := bufio.NewReader(stdin)
	if def != "" {
		fmt.Fprintf(stderr, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(stderr, "%s: ", label)
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

// promptStorage asks for the storage backend choice (file or keyring).
// Default is file (option 1).
func promptStorage(stdin io.Reader, stderr io.Writer) bool {
	r := bufio.NewReader(stdin)
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "Storage:")
	fmt.Fprintln(stderr, "  [1] Plain file ~/.config/llm-recall/credentials.toml (chmod 600) — default")
	fmt.Fprintln(stderr, "  [2] System keyring")
	fmt.Fprintln(stderr, "")
	for {
		fmt.Fprint(stderr, "Choose [1/2]: ")
		line, _ := r.ReadString('\n')
		switch strings.TrimSpace(line) {
		case "", "1":
			return false
		case "2":
			return true
		default:
			fmt.Fprintln(stderr, "  (please enter 1 or 2)")
		}
	}
}

// readAPIKey resolves the key per the rules at the top of this file.
// Hidden interactive input goes through term.ReadPassword; stdin pipes
// take a single line; non-tty stdout without --pipe-key is a hard
// error so we never accidentally echo the prompt to a log.
func readAPIKey(stdinIsPipe, pipeKey, stdoutIsTTY bool) (string, error) {
	if pipeKey || (stdinIsPipe && !stdoutIsTTY) {
		// Read one line from stdin, strip newline.
		buf, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		s := strings.TrimRight(string(buf), "\r\n")
		// If multi-line, take the first non-empty line (some shells
		// prepend trailing newlines).
		if i := strings.IndexAny(s, "\r\n"); i >= 0 {
			s = s[:i]
		}
		return s, nil
	}
	if !stdoutIsTTY {
		return "", errors.New("no API key on stdin; pass via `echo $KEY | llm-recall login --pipe-key` or run interactively")
	}
	fmt.Fprint(os.Stderr, "API key (input hidden): ")
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // newline after hidden read
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(pw), nil
}

// isStdinTTY / isStdoutTTY — small helpers that wrap term.IsTerminal
// so the callers above stay readable.
func isStdinTTY() bool  { return term.IsTerminal(int(os.Stdin.Fd())) }
func isStdoutTTY() bool { return term.IsTerminal(int(os.Stdout.Fd())) }
