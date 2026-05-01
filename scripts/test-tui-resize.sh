#!/usr/bin/env bash
# e2e TUI resize check.
#
# Walks the binary through 5 fake terminal sizes and writes the harness
# snapshot for each. The HOTFIX harness honours
# LLM_RECALL_TEST_TERM_WIDTH / _HEIGHT to seed an artificial WindowSizeMsg
# without needing a real PTY — same code path as a SIGWINCH from a live
# terminal.
#
# Usage:  scripts/test-tui-resize.sh
#         (must be run from the repo root with llm-recall.exe built)

set -u

BIN=./llm-recall.exe
[[ -x "$BIN" ]] || { echo "build llm-recall.exe first: go build -o llm-recall.exe ./cmd/llm-recall"; exit 1; }

OUTDIR=${TMPDIR:-/tmp}/llm-recall-resize-$$
mkdir -p "$OUTDIR"

declare -a SIZES=(
  "100 30"
  "80 24"
  "132 40"
  "60 16"
  "50 12"
)

for size in "${SIZES[@]}"; do
  read -r W H <<<"$size"
  SNAP="$OUTDIR/snap-${W}x${H}.txt"
  STDOUT="$OUTDIR/stdout-${W}x${H}.txt"
  echo "=== ${W}x${H} ==="
  LLM_RECALL_TEST_INPUT="\\e" \
  LLM_RECALL_TEST_TERM_WIDTH="$W" \
  LLM_RECALL_TEST_TERM_HEIGHT="$H" \
  LLM_RECALL_TEST_OUTPUT="$SNAP" \
    "$BIN" >"$STDOUT" 2>&1 || true
  if [[ -s "$SNAP" ]]; then
    head -10 "$SNAP"
  else
    echo "(snapshot empty — see $STDOUT)"
    head -5 "$STDOUT"
  fi
  echo
done

echo "Snapshots in $OUTDIR"
