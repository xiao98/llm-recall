package stats

import (
	"os/exec"
	"runtime"
)

// OpenInExplorer pops up the OS file browser at `path`. Best-effort: any
// error means the user just has to click the printed path themselves, so
// we swallow the error rather than crashing the CLI.
//
// Why not use Go's exec.LookPath: each OS has its own shell-style "open
// this thing" command (`explorer`, `open`, `xdg-open`); LookPath only
// finds binaries.
func OpenInExplorer(path string) error {
	switch runtime.GOOS {
	case "windows":
		// `explorer /select,<path>` would open the parent and pre-select
		// the file, but on Windows it can return non-zero even on success;
		// `explorer <path>` opens whatever the path points at (file → its
		// parent folder selected, dir → opens that dir). We pass the
		// parent dir because the user normally wants both PNGs visible.
		return exec.Command("cmd.exe", "/C", "explorer.exe", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
