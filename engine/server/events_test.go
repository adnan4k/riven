package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stream connects to /api/events and returns a channel of received events.
func stream(t *testing.T, h http.Handler) <-chan Event {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}

	events := make(chan Event, 32)
	go func() {
		defer resp.Body.Close()
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				var e Event
				json.Unmarshal([]byte(data), &e)
				events <- e
			}
		}
		close(events)
	}()
	time.Sleep(20 * time.Millisecond) // connected
	return events
}

func next(t *testing.T, events <-chan Event, want func(Event) bool, what string) Event {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case e, ok := <-events:
			if !ok {
				t.Fatalf("stream closed while waiting for %s", what)
			}
			if want(e) {
				return e
			}
		case <-timeout:
			t.Fatalf("no %s event", what)
		}
	}
}

// fakeApp is a directory with an artisan script that "exports" by writing the
// manifest, or fails when routes/broken.php exists.
func fakeApp(t *testing.T) (dir, manifestPath string) {
	t.Helper()
	if _, err := exec.LookPath("php"); err != nil {
		t.Skip("php not installed")
	}
	dir = t.TempDir()
	os.MkdirAll(filepath.Join(dir, "routes"), 0o755)
	os.WriteFile(filepath.Join(dir, "routes/web.php"), []byte("<?php"), 0o644)
	os.WriteFile(filepath.Join(dir, "artisan"), []byte(`<?php
if (file_exists(__DIR__.'/routes/broken.php')) { fwrite(STDERR, "Parse error in routes/broken.php"); exit(1); }
$out = substr(current(array_filter($argv, fn ($a) => str_starts_with($a, '--output='))), strlen('--output='));
file_put_contents($out, json_encode(['routes' => glob(__DIR__.'/routes/*.php'), 'at' => microtime(true)]));
echo "Wrote $out";`), 0o644)
	return dir, filepath.Join(dir, "riven.json")
}

func TestManifestChangesArePushed(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "riven.json")
	os.WriteFile(manifestPath, []byte("{}"), 0o644)
	sv := New(Config{ManifestPath: manifestPath, ReportsDir: t.TempDir(), UI: ui})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sv.Watch(ctx)
	events := stream(t, sv)
	time.Sleep(600 * time.Millisecond) // first scan of the manifest

	// Someone ran riven:export by hand.
	os.WriteFile(manifestPath, []byte(`{"schema_version":1}`), 0o644)

	next(t, events, func(e Event) bool { return e.Type == "manifest" }, "manifest")
}

func TestNewRouteFileTriggersExportAndPush(t *testing.T) {
	dir, manifestPath := fakeApp(t)
	sv := New(Config{ManifestPath: manifestPath, ReportsDir: t.TempDir(), UI: ui, AppDir: dir, AutoExport: true})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sv.Watch(ctx)
	events := stream(t, sv)
	time.Sleep(1200 * time.Millisecond) // first scans

	os.WriteFile(filepath.Join(dir, "routes/api.php"), []byte("<?php Route::get('/orders', fn () => []);"), 0o644)

	next(t, events, func(e Event) bool { return e.Type == "scan" && e.State == "running" }, "scan running")
	done := next(t, events, func(e Event) bool { return e.Type == "scan" && e.State != "running" }, "scan finished")
	if done.State != "done" || !strings.Contains(done.Message, "Wrote") {
		t.Fatalf("scan = %+v", done)
	}
	next(t, events, func(e Event) bool { return e.Type == "manifest" }, "manifest")

	data, _ := os.ReadFile(manifestPath)
	if !strings.Contains(string(data), "api.php") {
		t.Errorf("manifest wasn't re-exported with the new route file: %s", data)
	}
}

func TestFailedExportIsPushed(t *testing.T) {
	dir, manifestPath := fakeApp(t)
	sv := New(Config{ManifestPath: manifestPath, ReportsDir: t.TempDir(), UI: ui, AppDir: dir, AutoExport: true})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sv.Watch(ctx)
	events := stream(t, sv)
	time.Sleep(1200 * time.Millisecond)

	os.WriteFile(filepath.Join(dir, "routes/broken.php"), []byte("<?php Route::get("), 0o644)

	failed := next(t, events, func(e Event) bool { return e.Type == "scan" && e.State != "running" }, "scan finished")
	if failed.State != "failed" || !strings.Contains(failed.Message, "Parse error in routes/broken.php") {
		t.Errorf("scan = %+v, want the export's error", failed)
	}
}

func TestNoAutoExportWithoutFlagOrArtisan(t *testing.T) {
	dir, manifestPath := fakeApp(t)
	sv := New(Config{ManifestPath: manifestPath, ReportsDir: t.TempDir(), UI: ui, AppDir: dir}) // AutoExport off
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sv.Watch(ctx)
	time.Sleep(1200 * time.Millisecond)

	os.WriteFile(filepath.Join(dir, "routes/api.php"), []byte("<?php"), 0o644)
	time.Sleep(2 * time.Second)

	if _, err := os.Stat(manifestPath); err == nil {
		t.Error("exported although AutoExport is off")
	}
}

func TestCloseEndsStreams(t *testing.T) {
	sv := New(Config{ManifestPath: sample, ReportsDir: t.TempDir(), UI: ui})
	events := stream(t, sv)

	sv.Close()

	select {
	case _, ok := <-events:
		if ok {
			t.Error("expected the stream to end")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stream still open after Close")
	}
}
