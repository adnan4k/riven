// Package report defines the JSON files riven-engine writes for other tools
// (the dashboard, CI). Field names carry their units: durations are float
// milliseconds in *_ms fields.
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/faysal/riven/engine/manifest"
	"github.com/faysal/riven/engine/stress"
)

// Version is the report format version. Additive changes keep it; renames
// or removals bump it.
const Version = 1

type Stress struct {
	ReportVersion int          `json:"report_version"`
	Kind          string       `json:"kind"` // "stress"
	Generator     Generator    `json:"generator"`
	StartedAt     time.Time    `json:"started_at"`
	FinishedAt    time.Time    `json:"finished_at"`
	Interrupted   bool         `json:"interrupted"` // stopped early with Ctrl-C
	Manifest      ManifestInfo `json:"manifest"`
	Config        Config       `json:"config"`
	Summary       Stats        `json:"summary"`
	Routes        []RouteStats `json:"routes"`
}

type Generator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ManifestInfo identifies the app and the manifest the run was based on.
type ManifestInfo struct {
	GeneratedAt time.Time `json:"generated_at"`
	AppName     *string   `json:"app_name"`
	AppURL      *string   `json:"app_url"`
}

type Config struct {
	BaseURL     string  `json:"base_url"`
	Concurrency int     `json:"concurrency"`
	Requests    int     `json:"requests"`    // 0 = no limit
	DurationMs  float64 `json:"duration_ms"` // 0 = no limit
	TimeoutMs   float64 `json:"timeout_ms"`
}

type RouteStats struct {
	RouteID string `json:"route_id"` // matches routes[].id in the manifest
	Method  string `json:"method"`
	URL     string `json:"url"`
	Stats
}

type Stats struct {
	Requests  int     `json:"requests"`
	ElapsedMs float64 `json:"elapsed_ms"`
	RPS       float64 `json:"rps"`
	// LatencyMs is null when no request got a response: 0 would read as "fast".
	LatencyMs *Latency       `json:"latency_ms"`
	Statuses  map[string]int `json:"statuses"` // "200" => count (JSON keys are strings)
	Errors    map[string]int `json:"errors"`   // transport error kind => count
	ErrorRate float64        `json:"error_rate"`
}

type Latency struct {
	Min  float64 `json:"min"`
	Mean float64 `json:"mean"`
	P50  float64 `json:"p50"`
	P90  float64 `json:"p90"`
	P95  float64 `json:"p95"`
	P99  float64 `json:"p99"`
	Max  float64 `json:"max"`
}

// Run describes one stress run, as the CLI saw it.
type Run struct {
	EngineVersion string
	Manifest      *manifest.Manifest
	BaseURL       string
	Config        stress.Config
	StartedAt     time.Time
	FinishedAt    time.Time
	Interrupted   bool
	Summary       stress.Summary
}

func NewStress(r Run) Stress {
	rep := Stress{
		ReportVersion: Version,
		Kind:          "stress",
		Generator:     Generator{Name: "riven-engine", Version: r.EngineVersion},
		StartedAt:     r.StartedAt.UTC(),
		FinishedAt:    r.FinishedAt.UTC(),
		Interrupted:   r.Interrupted,
		Manifest: ManifestInfo{
			GeneratedAt: r.Manifest.GeneratedAt,
			AppName:     r.Manifest.App.Name,
			AppURL:      r.Manifest.App.URL,
		},
		Config: Config{
			BaseURL:     r.BaseURL,
			Concurrency: r.Config.Concurrency,
			Requests:    r.Config.Requests,
			DurationMs:  millis(r.Config.Duration),
			TimeoutMs:   millis(r.Config.Timeout),
		},
		Summary: stats(r.Summary.Total),
		Routes:  []RouteStats{}, // [] rather than null when empty
	}
	for _, t := range r.Summary.Targets {
		rep.Routes = append(rep.Routes, RouteStats{RouteID: t.Name, Method: t.Method, URL: t.URL, Stats: stats(t.Stats)})
	}
	return rep
}

func stats(s stress.Stats) Stats {
	out := Stats{
		Requests:  s.Requests,
		ElapsedMs: millis(s.Elapsed),
		RPS:       round(s.RPS),
		Statuses:  map[string]int{},
		Errors:    map[string]int{},
		ErrorRate: round(s.ErrorRate),
	}
	for code, n := range s.Statuses {
		out.Statuses[strconv.Itoa(code)] = n
	}
	for kind, n := range s.Errors {
		out.Errors[kind] = n
	}

	if len(s.Statuses) > 0 { // at least one response, so latency was measured
		l := s.Latency
		out.LatencyMs = &Latency{
			Min: millis(l.Min), Mean: millis(l.Mean), P50: millis(l.P50), P90: millis(l.P90),
			P95: millis(l.P95), P99: millis(l.P99), Max: millis(l.Max),
		}
	}
	return out
}

// millis converts to milliseconds with microsecond precision (3 decimals).
func millis(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

func round(f float64) float64 {
	return float64(int64(f*1000+0.5)) / 1000
}

// Write saves the report as indented JSON.
func Write(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}
