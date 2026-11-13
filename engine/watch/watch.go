// Package watch detects file changes by polling modification times. Polling
// needs no OS-specific code or dependency and is cheap at Laravel-app sizes
// (a few thousand files, stat'ed about once a second).
package watch

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type Watcher struct {
	Root     string   // directory the others are relative to
	Dirs     []string // directories to scan recursively, relative to Root (missing ones are fine)
	Files    []string // single files to watch, relative to Root
	Ext      string   // only files with this extension count; "" = all
	Skip     []string // directory names never entered (vendor, node_modules…) or paths relative to Root
	Interval time.Duration
	Quiet    time.Duration // wait for this long without changes before reporting (debounce)
}

type stamp struct {
	mod  time.Time
	size int64
}

// Run calls onChange once per burst of changes, until ctx is done.
func (w Watcher) Run(ctx context.Context, onChange func()) {
	interval := w.Interval
	if interval <= 0 {
		interval = time.Second
	}
	last := w.scan()
	var pendingSince time.Time

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		now := w.scan()
		if !same(last, now) {
			last = now
			pendingSince = time.Now()
			continue // wait for a quiet tick before reporting
		}
		if !pendingSince.IsZero() && time.Since(pendingSince) >= w.Quiet {
			pendingSince = time.Time{}
			onChange()
		}
	}
}

func (w Watcher) scan() map[string]stamp {
	files := map[string]stamp{}
	add := func(path string, info fs.FileInfo) {
		if w.Ext == "" || strings.HasSuffix(path, w.Ext) {
			files[path] = stamp{info.ModTime(), info.Size()}
		}
	}

	for _, f := range w.Files {
		if info, err := os.Stat(filepath.Join(w.Root, f)); err == nil {
			files[f] = stamp{info.ModTime(), info.Size()}
		}
	}
	for _, dir := range w.Dirs {
		filepath.WalkDir(filepath.Join(w.Root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // vanished or unreadable: skip, the next scan sees the new state
			}
			rel, _ := filepath.Rel(w.Root, path)
			if d.IsDir() {
				if slices.Contains(w.Skip, d.Name()) || slices.Contains(w.Skip, filepath.ToSlash(rel)) {
					return filepath.SkipDir
				}
				return nil
			}
			if info, err := d.Info(); err == nil {
				add(rel, info)
			}
			return nil
		})
	}
	return files
}

func same(a, b map[string]stamp) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || !w.mod.Equal(v.mod) || w.size != v.size {
			return false
		}
	}
	return true
}
