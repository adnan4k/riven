package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/faysal/riven/engine/report"
)

const sample = "../testdata/riven-app.json"

var ui = fstest.MapFS{
	"index.html":         {Data: []byte("<div id=root></div>")},
	"assets/app-abc1.js": {Data: []byte("console.log(1)")},
}

func get(t *testing.T, h http.Handler, path string) (int, http.Header, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	body, _ := io.ReadAll(rec.Body)
	return rec.Code, rec.Header(), string(body)
}

func writeReport(t *testing.T, dir, name string, started time.Time, requests int) {
	t.Helper()
	rep := report.Stress{ReportVersion: report.Version, Kind: "stress", StartedAt: started,
		Summary: report.Stats{Requests: requests}, Routes: []report.RouteStats{{RouteID: "GET /"}}}
	data, _ := json.Marshal(rep)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestManifestAndOpenAPI(t *testing.T) {
	h := New(Config{ManifestPath: sample, ReportsDir: t.TempDir(), UI: ui})

	code, hdr, body := get(t, h, "/api/manifest")
	if code != 200 || hdr.Get("Content-Type") != "application/json" || !strings.Contains(body, `"schema_version":1`) {
		t.Errorf("manifest: %d %s", code, body[:min(80, len(body))])
	}
	code, _, body = get(t, h, "/api/openapi")
	if code != 200 || !strings.Contains(body, `"openapi":"3.1.0"`) {
		t.Errorf("openapi: %d", code)
	}
}

func TestMissingManifestIs503WithHint(t *testing.T) {
	h := New(Config{ManifestPath: filepath.Join(t.TempDir(), "riven.json"), ReportsDir: t.TempDir(), UI: ui})

	code, _, body := get(t, h, "/api/manifest")
	if code != http.StatusServiceUnavailable || !strings.Contains(body, "riven:export") {
		t.Errorf("got %d %s", code, body)
	}
}

func TestReportsListNewestFirstAndSkipsOtherFiles(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	writeReport(t, dir, "old.json", t0, 10)
	writeReport(t, dir, "new.json", t0.Add(time.Hour), 20)
	os.WriteFile(filepath.Join(dir, "openapi.json"), []byte(`{"openapi":"3.1.0"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644)

	code, _, body := get(t, New(Config{ManifestPath: sample, ReportsDir: dir, UI: ui}), "/api/reports")

	var list []ReportSummary
	json.Unmarshal([]byte(body), &list)
	if code != 200 || len(list) != 2 || list[0].Name != "new.json" || list[0].Summary.Requests != 20 || list[1].Routes != 1 {
		t.Errorf("got %d %+v", code, list)
	}
}

func TestReportsDirMayNotExistYet(t *testing.T) {
	h := New(Config{ManifestPath: sample, ReportsDir: filepath.Join(t.TempDir(), "nope"), UI: ui})

	if code, _, body := get(t, h, "/api/reports"); code != 200 || strings.TrimSpace(body) != "[]" {
		t.Errorf("got %d %q, want 200 []", code, body)
	}
}

func TestOneReport(t *testing.T) {
	dir := t.TempDir()
	writeReport(t, dir, "run.json", time.Now(), 5)
	h := New(Config{ManifestPath: sample, ReportsDir: dir, UI: ui})

	if code, _, body := get(t, h, "/api/reports/run.json"); code != 200 || !strings.Contains(body, `"route_id":"GET /"`) {
		t.Errorf("got %d %s", code, body)
	}
	if code, _, _ := get(t, h, "/api/reports/missing.json"); code != 404 {
		t.Errorf("missing report: %d", code)
	}
}

func TestReportNamesCantEscapeTheDirectory(t *testing.T) {
	base := t.TempDir()
	reports := filepath.Join(base, "reports")
	os.Mkdir(reports, 0o755)
	writeReport(t, base, "secret.json", time.Now(), 1) // a valid report, but outside the dir

	h := New(Config{ManifestPath: sample, ReportsDir: reports, UI: ui})
	for _, path := range []string{"/api/reports/..%2Fsecret.json", "/api/reports/%2E%2E%2Fsecret.json"} {
		if code, _, body := get(t, h, path); code != 404 || strings.Contains(body, "stress") && !strings.Contains(body, "no stress report") {
			t.Errorf("%s: got %d %s", path, code, body)
		}
	}
}

func TestUnknownAPIPathIsJSON404(t *testing.T) {
	code, _, body := get(t, New(Config{ManifestPath: sample, ReportsDir: t.TempDir(), UI: ui}), "/api/nope")
	if code != 404 || !strings.Contains(body, `"error"`) {
		t.Errorf("got %d %s", code, body)
	}
}

func TestServesUIWithClientSideRoutes(t *testing.T) {
	h := New(Config{ManifestPath: sample, ReportsDir: t.TempDir(), UI: ui})

	for _, path := range []string{"/", "/routes/GET%20%2Fapi%2Fusers", "/reports/run.json"} {
		if code, _, body := get(t, h, path); code != 200 || body != "<div id=root></div>" {
			t.Errorf("%s: got %d %q, want index.html", path, code, body)
		}
	}
	code, hdr, body := get(t, h, "/assets/app-abc1.js")
	if code != 200 || body != "console.log(1)" || !strings.Contains(hdr.Get("Cache-Control"), "immutable") {
		t.Errorf("asset: %d %q %v", code, body, hdr)
	}
}

func TestUnbuiltUIShowsBuildHint(t *testing.T) {
	h := New(Config{ManifestPath: sample, ReportsDir: t.TempDir(), UI: fstest.MapFS{}})

	code, _, body := get(t, h, "/")
	if code != http.StatusServiceUnavailable || !strings.Contains(body, "npm run build") {
		t.Errorf("got %d %s", code, body)
	}
	if code, _, _ := get(t, h, "/api/manifest"); code != 200 {
		t.Errorf("API should work without the UI, got %d", code)
	}
}
