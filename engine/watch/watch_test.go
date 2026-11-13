package watch

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func start(t *testing.T, w Watcher) *atomic.Int64 {
	t.Helper()
	var calls atomic.Int64
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go w.Run(ctx, func() { calls.Add(1) })
	time.Sleep(30 * time.Millisecond) // let the first scan happen
	return &calls
}

func write(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func eventually(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatal(msg)
}

func watcher(root string) Watcher {
	return Watcher{
		Root: root, Dirs: []string{"routes", "app"}, Ext: ".php",
		Skip: []string{"vendor", "app/cache"}, Interval: 10 * time.Millisecond, Quiet: 30 * time.Millisecond,
	}
}

func TestReportsANewFileOncePerBurst(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "routes/web.php"), "<?php")
	calls := start(t, watcher(root))

	// Three quick writes (an editor saving) are one change.
	for i := range 3 {
		write(t, filepath.Join(root, "routes/api.php"), "<?php // "+string(rune('a'+i)))
		time.Sleep(5 * time.Millisecond)
	}

	eventually(t, func() bool { return calls.Load() == 1 }, "expected one change report")
	time.Sleep(100 * time.Millisecond)
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1 for one burst", calls.Load())
	}
}

func TestReportsEditsAndDeletesInNestedDirs(t *testing.T) {
	root := t.TempDir()
	ctrl := filepath.Join(root, "app/Http/Controllers/UserController.php")
	write(t, ctrl, "<?php class A {}")
	calls := start(t, watcher(root))

	write(t, ctrl, "<?php class A { public function store() {} }")
	eventually(t, func() bool { return calls.Load() == 1 }, "edit not reported")

	os.Remove(ctrl)
	eventually(t, func() bool { return calls.Load() == 2 }, "delete not reported")
}

func TestIgnoresOtherFilesAndSkippedDirs(t *testing.T) {
	root := t.TempDir()
	calls := start(t, watcher(root))

	write(t, filepath.Join(root, "routes/notes.txt"), "x")         // wrong extension
	write(t, filepath.Join(root, "app/cache/routes.php"), "<?php") // skipped path
	write(t, filepath.Join(root, "app/vendor/x.php"), "<?php")     // skipped dir name
	write(t, filepath.Join(root, "storage/x.php"), "<?php")        // not watched

	time.Sleep(150 * time.Millisecond)
	if calls.Load() != 0 {
		t.Errorf("calls = %d, want 0", calls.Load())
	}
}

func TestWatchesSingleFiles(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "riven.json"), "{}")
	calls := start(t, Watcher{Root: root, Files: []string{"riven.json"}, Interval: 10 * time.Millisecond, Quiet: 20 * time.Millisecond})

	write(t, filepath.Join(root, "riven.json"), `{"schema_version":1}`)
	eventually(t, func() bool { return calls.Load() == 1 }, "file change not reported")
}

func TestStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { watcher(t.TempDir()).Run(ctx, func() {}); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run didn't return after cancel")
	}
}
