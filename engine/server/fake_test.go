package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeArtisan writes an artisan script whose riven:fake-body prints numbered
// bodies (after some noise, like a deprecation notice) or an error.
func fakeArtisan(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("php"); err != nil {
		t.Skip("php not installed")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "artisan"), []byte(`<?php
if ($argv[1] !== 'riven:fake-body') { exit(1); }
$route = $argv[2];
$count = (int) substr(current(array_filter($argv, fn ($a) => str_starts_with($a, '--count='))), strlen('--count='));
echo "PHP Deprecated: something noisy\n";
if (! str_starts_with($route, 'POST')) { echo json_encode(['error' => "$route has no FormRequest"]), "\n"; exit(1); }
$bodies = array_map(fn ($i) => ['email' => "user$i@example.com"], range(1, $count));
echo json_encode(['route_id' => $route, 'form_request' => 'App\\Req', 'bodies' => $bodies, 'invalid' => 0, 'errors' => new stdClass, 'warnings' => []]), "\n";
`), 0o644)
	return dir
}

func TestFakeBodyEndpoint(t *testing.T) {
	appSrv, _ := app(t, 0)
	h := New(Config{ManifestPath: sample, ReportsDir: t.TempDir(), UI: ui, BaseURL: appSrv.URL, AppDir: fakeArtisan(t)})

	code, body := post(t, h, "/api/fake", `{"route_id":"POST /api/users","count":3}`)
	var res FakeResult
	json.Unmarshal([]byte(body), &res)
	if code != 200 || len(res.Bodies) != 3 || !strings.Contains(string(res.Bodies[2]), "user3@example.com") {
		t.Fatalf("got %d %s", code, body)
	}

	if code, body := post(t, h, "/api/fake", `{"route_id":"GET /api/users"}`); code != http.StatusUnprocessableEntity || !strings.Contains(body, "has no FormRequest") {
		t.Errorf("error from the command should be passed on: %d %s", code, body)
	}
	if code, _ := post(t, h, "/api/fake", `{"route_id":"POST /api/users","count":501}`); code != http.StatusBadRequest {
		t.Errorf("count over the limit: got %d", code)
	}
}

func TestFakeBodyNeedsTheApp(t *testing.T) {
	h := New(Config{ManifestPath: sample, ReportsDir: t.TempDir(), UI: ui, AppDir: t.TempDir()})
	if code, body := post(t, h, "/api/fake", `{"route_id":"POST /api/users"}`); code != http.StatusUnprocessableEntity || !strings.Contains(body, "needs the Laravel app") {
		t.Errorf("got %d %s", code, body)
	}
}

func TestStressWithGeneratedBodies(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	var contentType string
	appSrv := newApp(t, func(r *http.Request, body string) {
		mu.Lock()
		seen[body]++
		contentType = r.Header.Get("Content-Type")
		mu.Unlock()
	})
	h := New(Config{ManifestPath: sample, ReportsDir: t.TempDir(), UI: ui, BaseURL: appSrv, AppDir: fakeArtisan(t)})

	code, body := post(t, h, "/api/stress", `{"routes":["POST /api/users"],"concurrency":1,"requests":6,"fake_bodies":3}`)
	if code != 200 {
		t.Fatalf("start: %d %s", code, body)
	}
	waitDone(t, h)

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 3 || contentType != "application/json" {
		t.Errorf("bodies seen = %v (content type %q), want 3 different bodies twice each", seen, contentType)
	}
	for b, n := range seen {
		if n != 2 || !strings.Contains(b, "@example.com") {
			t.Errorf("body %s sent %d times", b, n)
		}
	}
}
