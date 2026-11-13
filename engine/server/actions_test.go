package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// app is a stand-in Laravel app.
func app(t *testing.T, delay time.Duration) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(delay)
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"method": r.Method, "path": r.URL.RequestURI(), "auth": r.Header.Get("Authorization"), "body": string(body),
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func post(t *testing.T, h http.Handler, path, body string, mutate ...func(*http.Request)) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Host = "127.0.0.1:7600"
	req.Header.Set("Content-Type", "application/json")
	for _, m := range mutate {
		m(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func newWithApp(t *testing.T, appURL string) (http.Handler, Config) {
	cfg := Config{ManifestPath: sample, ReportsDir: t.TempDir(), UI: ui, BaseURL: appURL, Version: "test"}
	return New(cfg), cfg
}

func TestGuardRejectsCrossSiteAndNonJSON(t *testing.T) {
	appSrv, hits := app(t, 0)
	h, _ := newWithApp(t, appSrv.URL)
	body := `{"method":"GET","path":"/api/users"}`

	cases := map[string]struct {
		mutate func(*http.Request)
		want   int
	}{
		"form post (no JSON)":   {func(r *http.Request) { r.Header.Set("Content-Type", "application/x-www-form-urlencoded") }, http.StatusUnsupportedMediaType},
		"other website":         {func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }, http.StatusForbidden},
		"DNS rebinding host":    {func(r *http.Request) { r.Host = "evil.example:7600" }, http.StatusForbidden},
		"same-origin dashboard": {func(r *http.Request) { r.Header.Set("Origin", "http://127.0.0.1:7600") }, http.StatusOK},
	}
	for name, tc := range cases {
		if code, body := post(t, h, "/api/try", body, tc.mutate); code != tc.want {
			t.Errorf("%s: got %d %s, want %d", name, code, body, tc.want)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("app got %d requests, want 1 (only the allowed one)", hits.Load())
	}
}

func TestTrySendsToTheAppAndReportsTiming(t *testing.T) {
	appSrv, _ := app(t, 5*time.Millisecond)
	h, _ := newWithApp(t, appSrv.URL)

	code, body := post(t, h, "/api/try", `{"method":"POST","path":"/api/users?x=1","headers":{"Authorization":"Bearer t"},"body":"{\"name\":\"a\"}"}`)
	if code != 200 {
		t.Fatalf("got %d %s", code, body)
	}
	var res tryResponse
	json.Unmarshal([]byte(body), &res)
	if res.Status != 201 || res.DurationMs < 5 || res.SizeBytes == 0 || res.Headers["Content-Type"] != "application/json" {
		t.Errorf("response = %+v", res)
	}
	for _, want := range []string{`"method":"POST"`, `"path":"/api/users?x=1"`, `"auth":"Bearer t"`, `"body":"{\"name\":\"a\"}"`} {
		if !strings.Contains(res.Body, want) {
			t.Errorf("app didn't receive %s: %s", want, res.Body)
		}
	}
}

func TestTryOnlyTargetsTheApp(t *testing.T) {
	appSrv, hits := app(t, 0)
	h, _ := newWithApp(t, appSrv.URL)

	for _, path := range []string{"//evil.example/x", "http://evil.example/x", "evil", `/\evil.example`} {
		if code, _ := post(t, h, "/api/try", `{"method":"GET","path":"`+strings.ReplaceAll(path, `\`, `\\`)+`"}`); code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", path, code)
		}
	}
	if code, _ := post(t, h, "/api/try", `{"method":"TRACE","path":"/"}`); code != http.StatusBadRequest {
		t.Errorf("TRACE: got %d, want 400", code)
	}
	if hits.Load() != 0 {
		t.Errorf("app got %d requests, want 0", hits.Load())
	}
}

func waitDone(t *testing.T, h http.Handler) StressState {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, _, body := get(t, h, "/api/stress")
		var st StressState
		json.Unmarshal([]byte(body), &st)
		if st.State == "done" || st.State == "failed" {
			return st
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("stress run didn't finish")
	return StressState{}
}

func TestStressRunFromDashboard(t *testing.T) {
	appSrv, hits := app(t, time.Millisecond)
	h, cfg := newWithApp(t, appSrv.URL)

	if _, _, body := get(t, h, "/api/stress"); !strings.Contains(body, `"state":"idle"`) {
		t.Errorf("before any run: %s", body)
	}

	code, body := post(t, h, "/api/stress", `{"routes":["GET /api/users","GET /api/users/{user}"],"params":{"user":"7"},"token":"secret","concurrency":2,"requests":20}`)
	if code != 200 || !strings.Contains(body, `"state":"running"`) && !strings.Contains(body, `"state":"done"`) {
		t.Fatalf("start: %d %s", code, body)
	}

	st := waitDone(t, h)
	if st.State != "done" || st.Live.Summary.Requests != 20 || hits.Load() != 20 || len(st.Live.Routes) != 2 {
		t.Fatalf("state = %+v (hits %d)", st, hits.Load())
	}
	if st.Live.Routes[1].URL != appSrv.URL+"/api/users/7" || len(st.Live.Timeline) == 0 {
		t.Errorf("routes = %+v, timeline = %v", st.Live.Routes, st.Live.Timeline)
	}
	if _, err := os.Stat(filepath.Join(cfg.ReportsDir, st.Report)); st.Report == "" || err != nil {
		t.Errorf("report %q not saved: %v", st.Report, err)
	}
	if _, _, list := get(t, h, "/api/reports"); !strings.Contains(list, st.Report) {
		t.Errorf("saved run missing from /api/reports: %s", list)
	}
}

func TestStressOneRunAtATimeAndStop(t *testing.T) {
	appSrv, _ := app(t, 20*time.Millisecond)
	h, _ := newWithApp(t, appSrv.URL)
	start := `{"routes":["GET /api/users"],"concurrency":2,"duration_s":30}`

	if code, body := post(t, h, "/api/stress", start); code != 200 {
		t.Fatalf("start: %d %s", code, body)
	}
	if code, _ := post(t, h, "/api/stress", start); code != http.StatusConflict {
		t.Errorf("second start: got %d, want 409", code)
	}
	time.Sleep(100 * time.Millisecond)
	if code, body := post(t, h, "/api/stress/stop", `{}`); code != 200 {
		t.Fatalf("stop: %d %s", code, body)
	}

	st := waitDone(t, h)
	if !st.Interrupted || st.Report == "" || st.Live.Summary.Requests == 0 {
		t.Errorf("stopped run = %+v", st)
	}
	if code, _ := post(t, h, "/api/stress/stop", `{}`); code != http.StatusConflict {
		t.Errorf("stop with nothing running: got %d, want 409", code)
	}
}

func TestStressRejectsBadInput(t *testing.T) {
	appSrv, _ := app(t, 0)
	h, _ := newWithApp(t, appSrv.URL)

	for _, body := range []string{
		`{"routes":[],"concurrency":1,"requests":1}`,
		`{"routes":["GET /api/users"],"concurrency":0,"requests":1}`,
		`{"routes":["GET /api/users"],"concurrency":1}`,
		`{"routes":["GET /api/users"],"concurrency":1,"duration_s":99999}`,
		`{"routes":["GET /nope"],"concurrency":1,"requests":1}`,
		`{"routes":["GET /api/users/{user}"],"concurrency":1,"requests":1}`, // missing param
	} {
		if code, _ := post(t, h, "/api/stress", body); code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", body, code)
		}
	}
}

func TestRescanRunsArtisanExport(t *testing.T) {
	if _, err := exec.LookPath("php"); err != nil {
		t.Skip("php not installed")
	}
	dir := t.TempDir()
	// A fake artisan that just writes the --output file it was given.
	os.WriteFile(filepath.Join(dir, "artisan"), []byte(`<?php
$out = substr($argv[3], strlen('--output='));
file_put_contents($out, '{"ok":true}');
echo "Wrote $out";`), 0o644)
	manifestPath := filepath.Join(dir, "riven.json")
	h := New(Config{ManifestPath: manifestPath, ReportsDir: t.TempDir(), UI: ui, AppDir: dir})

	code, body := post(t, h, "/api/rescan", `{}`)
	if code != 200 || !strings.Contains(body, "Wrote") {
		t.Fatalf("got %d %s", code, body)
	}
	if data, _ := os.ReadFile(manifestPath); string(data) != `{"ok":true}` {
		t.Errorf("manifest = %s", data)
	}

	noApp := New(Config{ManifestPath: manifestPath, ReportsDir: t.TempDir(), UI: ui, AppDir: t.TempDir()})
	if code, _ := post(t, noApp, "/api/rescan", `{}`); code != http.StatusNotFound {
		t.Errorf("no artisan: got %d, want 404", code)
	}
}

func TestConfigReportsBaseURL(t *testing.T) {
	h, _ := newWithApp(t, "http://127.0.0.1:9999")
	if _, _, body := get(t, h, "/api/config"); !strings.Contains(body, `"base_url":"http://127.0.0.1:9999"`) {
		t.Errorf("config = %s", body)
	}
	h2 := New(Config{ManifestPath: sample, ReportsDir: t.TempDir(), UI: ui})
	if _, _, body := get(t, h2, "/api/config"); !strings.Contains(body, `"base_url":"http://localhost:8000"`) {
		t.Errorf("default base URL should be the manifest's app.url: %s", body)
	}
}
