package report

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/faysal/riven/engine/manifest"
	"github.com/faysal/riven/engine/stress"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata/")

var t0 = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

func ms(n float64) time.Duration { return time.Duration(n * float64(time.Millisecond)) }

func fixedRun() Run {
	targets := []stress.Target{
		{Name: "GET /api/users", Method: "GET", URL: "http://localhost:8000/api/users"},
		{Name: "GET /api/users/{user}", Method: "GET", URL: "http://localhost:8000/api/users/1"},
	}
	results := []stress.Result{
		{Target: 0, Start: t0, Latency: ms(12.3456), Status: 200},
		{Target: 0, Start: t0.Add(ms(20)), Latency: ms(30), Status: 200},
		{Target: 0, Start: t0.Add(ms(60)), Latency: ms(40), Status: 500},
		// Only a timeout: this route has no latency samples.
		{Target: 1, Start: t0, Latency: ms(100), Err: context.DeadlineExceeded},
	}
	name, url := "riven-app", "http://localhost:8000"

	return Run{
		EngineVersion: "dev",
		Manifest:      &manifest.Manifest{GeneratedAt: t0.Add(-time.Hour), App: manifest.App{Name: &name, URL: &url}},
		BaseURL:       url,
		Config:        stress.Config{Concurrency: 2, Duration: 3 * time.Second, Timeout: 30 * time.Second},
		StartedAt:     t0,
		FinishedAt:    t0.Add(ms(100)),
		Summary:       stress.Summarize(targets, results),
	}
}

func TestStressReportGolden(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := Write(path, NewStress(fixedRun())); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)

	golden := filepath.Join("testdata", "stress.golden.json")
	if *update {
		os.WriteFile(golden, got, 0o644)
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run `go test ./report -update` to create it)", err)
	}
	if string(got) != string(want) {
		t.Errorf("report mismatch (run with -update if intended)\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestStressReportShape(t *testing.T) {
	data, _ := json.Marshal(NewStress(fixedRun()))
	var raw map[string]any
	json.Unmarshal(data, &raw)

	routes := raw["routes"].([]any)
	timedOut := routes[1].(map[string]any)
	if timedOut["latency_ms"] != nil {
		t.Errorf("route with no responses: latency_ms = %v, want null", timedOut["latency_ms"])
	}
	if statuses := timedOut["statuses"].(map[string]any); len(statuses) != 0 {
		t.Errorf("statuses = %v, want {} (not null)", statuses)
	}

	users := routes[0].(map[string]any)
	if users["route_id"] != "GET /api/users" || users["latency_ms"].(map[string]any)["min"] != 12.345 {
		t.Errorf("users route = %v", users)
	}
}

func TestEmptyRunHasEmptyRoutesArray(t *testing.T) {
	r := fixedRun()
	r.Summary = stress.Summarize(nil, nil)

	data, _ := json.Marshal(NewStress(r))
	var raw map[string]any
	json.Unmarshal(data, &raw)

	if routes, ok := raw["routes"].([]any); !ok || len(routes) != 0 {
		t.Errorf("routes = %#v, want []", raw["routes"])
	}
}
