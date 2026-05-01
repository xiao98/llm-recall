package adapter

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CandidateFile is one on-disk session blob — path + the (mtime, size) tuple
// the cache uses to decide whether a re-parse is needed.
type CandidateFile struct {
	Path  string
	Mtime int64
	Size  int64
}

// FileLister is an optional capability: an adapter that can enumerate its
// session files without parsing them. The cache layer needs this so it can
// diff disk-vs-DB cheaply. Adapters that don't implement it always re-parse.
type FileLister interface {
	ListFiles() ([]CandidateFile, error)
}

// FileParser is an optional capability: parse exactly one session file. The
// cache layer calls this on a miss instead of running the full Discover().
type FileParser interface {
	ParseFile(path string) (Session, error)
}

// listFilesGeneric walks `root` and yields every entry that satisfies `match`.
// Returns nil (no error) when root is missing — callers treat that as "vendor
// not installed".
func listFilesGeneric(root string, match func(path string, name string) bool) ([]CandidateFile, error) {
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}
	var out []CandidateFile
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			// Skip unreadable subtrees but keep walking.
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !match(path, d.Name()) {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		out = append(out, CandidateFile{
			Path:  path,
			Mtime: fi.ModTime().Unix(),
			Size:  fi.Size(),
		})
		return nil
	})
	return out, err
}

// ----- per-adapter implementations -----

// ListFiles for Claude: <root>/<project-dir>/*.jsonl, no recursion below the
// project dir (matches Discover()'s rule about not pulling subagent files).
func (c *Claude) ListFiles() ([]CandidateFile, error) {
	root, err := c.projectsRoot()
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}
	projectDirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []CandidateFile
	for _, pd := range projectDirs {
		if !pd.IsDir() {
			continue
		}
		projectPath := filepath.Join(root, pd.Name())
		entries, err := os.ReadDir(projectPath)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if filepath.Ext(e.Name()) != ".jsonl" {
				continue
			}
			fi, err := e.Info()
			if err != nil {
				continue
			}
			out = append(out, CandidateFile{
				Path:  filepath.Join(projectPath, e.Name()),
				Mtime: fi.ModTime().Unix(),
				Size:  fi.Size(),
			})
		}
	}
	return out, nil
}

// ListFiles for Codex: recursive walk of YYYY/MM/DD picking up rollout-*.jsonl.
func (c *Codex) ListFiles() ([]CandidateFile, error) {
	root, err := c.sessionsRoot()
	if err != nil {
		return nil, err
	}
	return listFilesGeneric(root, func(_ string, name string) bool {
		return strings.HasPrefix(name, "rollout-") && filepath.Ext(name) == ".jsonl"
	})
}

// ListFiles for Gemini: <root>/<project>/chats/session-*.{json,jsonl}, only
// directly under chats/ (the nested rewind-snapshot dirs are skipped).
func (g *Gemini) ListFiles() ([]CandidateFile, error) {
	root, err := g.tmpRoot()
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}
	projectDirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []CandidateFile
	for _, pd := range projectDirs {
		if !pd.IsDir() {
			continue
		}
		chatsDir := filepath.Join(root, pd.Name(), "chats")
		entries, err := os.ReadDir(chatsDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasPrefix(name, "session-") {
				continue
			}
			ext := filepath.Ext(name)
			if ext != ".json" && ext != ".jsonl" {
				continue
			}
			fi, err := e.Info()
			if err != nil {
				continue
			}
			out = append(out, CandidateFile{
				Path:  filepath.Join(chatsDir, name),
				Mtime: fi.ModTime().Unix(),
				Size:  fi.Size(),
			})
		}
	}
	return out, nil
}
