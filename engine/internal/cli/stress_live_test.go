package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/adnan4k/riven/engine/server"
)

func readLive(t *testing.T, path string) server.StressState {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var st server.StressState
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestStressLiveFileEndsDoneWithTheReport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	dir := t.TempDir()
	live, out := filepath.Join(dir, "live.json"), filepath.Join(dir, "stress-1.json")

	if _, _, err := run(t, "stress", "-m", sampleManifest, "--base-url", srv.URL, "-n", "4", "-c", "2", "-o", out, "--live-file", live); err != nil {
		t.Fatal(err)
	}

	st := readLive(t, live)
	if st.State != "done" || st.Report != "stress-1.json" || st.Live == nil || st.Live.Summary.Requests != 4 || st.ID == "" {
		t.Errorf("live state = %+v", st)
	}
}

func TestStressStopsWhenTheStopFileAppears(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	dir := t.TempDir()
	live, stop, out := filepath.Join(dir, "live.json"), filepath.Join(dir, "stop"), filepath.Join(dir, "r.json")

	go func() {
		time.Sleep(300 * time.Millisecond)
		os.WriteFile(stop, nil, 0o644)
	}()
	start := time.Now()
	if _, _, err := run(t, "stress", "-m", sampleManifest, "--base-url", srv.URL, "-d", "30s", "-o", out, "--live-file", live, "--stop-file", stop); err != nil {
		t.Fatal(err)
	}

	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("run took %s, the stop file didn't stop it", took)
	}
	if st := readLive(t, live); st.State != "done" || !st.Interrupted || st.Report != "r.json" {
		t.Errorf("live state = %+v", st)
	}
}

func TestStressRotatesBodiesFromTheBodiesFile(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]string{} // body => content type
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen[string(b)] = r.Header.Get("Content-Type")
		mu.Unlock()
	}))
	defer srv.Close()
	bodies := filepath.Join(t.TempDir(), "bodies.json")
	os.WriteFile(bodies, []byte(`{"POST /api/users": [{"n":1}, {"n":2}]}`), 0o644)

	if _, _, err := run(t, "stress", "-m", sampleManifest, "--base-url", srv.URL, "--route", "POST /api/users", "-n", "4", "-c", "1", "--bodies-file", bodies); err != nil {
		t.Fatal(err)
	}

	if seen[`{"n":1}`] != "application/json" || seen[`{"n":2}`] != "application/json" {
		t.Errorf("bodies seen = %v", seen)
	}
}

func TestExtractUIWritesTheDashboard(t *testing.T) {
	ui := fstest.MapFS{
		"index.html":    {Data: []byte("<html>")},
		"assets/app.js": {Data: []byte("js")},
		".gitkeep":      {Data: nil},
	}
	dir := t.TempDir()

	n, err := extractUI(ui, dir)
	if err != nil || n != 2 {
		t.Fatalf("extractUI = %d, %v", n, err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "assets", "app.js")); string(data) != "js" {
		t.Errorf("assets/app.js = %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitkeep")); err == nil {
		t.Error(".gitkeep was copied")
	}
}

func TestExtractUIRefusesABinaryWithoutTheDashboard(t *testing.T) {
	if _, err := extractUI(fstest.MapFS{".gitkeep": {}}, t.TempDir()); err == nil {
		t.Fatal("want an error when index.html is missing")
	}
}
