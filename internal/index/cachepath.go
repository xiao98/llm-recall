package index

import (
	"os"
	"path/filepath"
	"runtime"
)

// CacheDBPath returns the absolute path of the SQLite cache file. The
// directory is platform-specific (XDG on unix, %LocalAppData% on Windows)
// and is intentionally OUTSIDE the user's repo so a `git clean` never wipes
// the cache. Caller is responsible for MkdirAll on the parent directory.
func CacheDBPath() (string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "llm-recall", "cache", "index.db"), nil
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "llm-recall", "index.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "llm-recall", "index.db"), nil
}
