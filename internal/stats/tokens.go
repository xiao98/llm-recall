package stats

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"
)

// TokensFromFile parses a single session file and returns total tokens
// consumed by that session. Source-specific layout — see backend/TOKEN-AUDIT.md
// for the field paths and aggregation rules:
//
//   - claude: assistant rows' message.usage.{input_tokens, output_tokens}
//     summed (cache_* are NOT counted).
//   - codex:  the LAST event_msg/token_count's payload.info.total_token_usage
//     .total_tokens (already cumulative).
//   - gemini: messages[i].tokens.total summed across messages with tokens.
//
// Returns (0, nil) — not an error — when the file exists but has no token
// fields. Caller treats that as "fall back to message count for this row".
func TokensFromFile(source, path string) (int64, error) {
	switch source {
	case "claude":
		return tokensClaude(path)
	case "codex":
		return tokensCodex(path)
	case "gemini":
		return tokensGemini(path)
	default:
		return 0, nil
	}
}

// tokensClaude scans an assistant row stream and adds up input + output.
// We deliberately ignore cache_creation_input_tokens / cache_read_input_tokens:
// W5 §6 / TOKEN-AUDIT.md decision is to count actually-spent tokens, not
// cache savings.
func tokensClaude(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()

	type usage struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	}
	type message struct {
		Usage *usage `json:"usage"`
	}
	type row struct {
		Type    string  `json:"type"`
		Message message `json:"message"`
	}

	var total int64
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r row
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		if r.Type != "assistant" || r.Message.Usage == nil {
			continue
		}
		total += r.Message.Usage.InputTokens + r.Message.Usage.OutputTokens
	}
	return total, nil
}

// tokensCodex tracks the LAST token_count event in the session. The field
// info.total_token_usage.total_tokens is already cumulative for the session,
// so we don't sum across rows — we replace.
func tokensCodex(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()

	type usage struct {
		Total int64 `json:"total_tokens"`
	}
	type info struct {
		TotalTokenUsage *usage `json:"total_token_usage"`
	}
	type payload struct {
		Type string `json:"type"`
		Info *info  `json:"info"`
	}
	type row struct {
		Type    string  `json:"type"`
		Payload payload `json:"payload"`
	}

	var last int64
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		// Cheap pre-filter: skip lines that can't possibly be a token_count
		// event. Saves ~30% on big files.
		if !strings.Contains(string(line), "token_count") {
			continue
		}
		var r row
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		if r.Type != "event_msg" || r.Payload.Type != "token_count" {
			continue
		}
		if r.Payload.Info == nil || r.Payload.Info.TotalTokenUsage == nil {
			continue
		}
		last = r.Payload.Info.TotalTokenUsage.Total
	}
	return last, nil
}

// tokensGemini reads the entire JSON document (it is one object, not jsonl)
// and sums each message's tokens.total.
func tokensGemini(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()

	type tokens struct {
		Total int64 `json:"total"`
		// Fall back to input+output if total is missing.
		Input  int64 `json:"input"`
		Output int64 `json:"output"`
	}
	type msg struct {
		Tokens *tokens `json:"tokens"`
	}
	type doc struct {
		Messages []msg `json:"messages"`
	}

	var d doc
	if err := json.NewDecoder(f).Decode(&d); err != nil {
		return 0, nil // tolerate bad/partial files
	}
	var total int64
	for _, m := range d.Messages {
		if m.Tokens == nil {
			continue
		}
		if m.Tokens.Total > 0 {
			total += m.Tokens.Total
		} else {
			total += m.Tokens.Input + m.Tokens.Output
		}
	}
	return total, nil
}
