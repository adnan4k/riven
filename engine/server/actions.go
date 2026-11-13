package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/faysal/riven/engine/manifest"
	"github.com/faysal/riven/engine/report"
	"github.com/faysal/riven/engine/stress"
	"github.com/faysal/riven/engine/targets"
)

// guard protects endpoints that act (send requests, run load, run artisan).
// The server listens on localhost, but any website open in the browser can
// still try to reach it:
//   - only JSON bodies are accepted, so a plain HTML form can't post here
//     (JSON makes the browser ask permission first, which we never grant);
//   - a request from another page (Origin header) must come from this host;
//   - the Host header must be a loopback name, which stops DNS rebinding
//     (a hostile domain re-pointed at 127.0.0.1), unless serving on a LAN on purpose.
func (s *srv) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "send JSON (Content-Type: application/json)")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			if u, err := url.Parse(origin); err != nil || u.Host != r.Host {
				writeError(w, http.StatusForbidden, "requests must come from the Riven dashboard itself")
				return
			}
		}
		if !s.cfg.AllowAnyHost && !isLoopback(r.Host) {
			writeError(w, http.StatusForbidden, "this endpoint only answers on localhost")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
		next(w, r)
	}
}

func isLoopback(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

// baseURL is --base-url, else the manifest's app.url.
func (s *srv) baseURL(m *manifest.Manifest) string {
	if s.cfg.BaseURL != "" {
		return s.cfg.BaseURL
	}
	if m.App.URL != nil {
		return *m.App.URL
	}
	return ""
}

// GET /api/config
func (s *srv) config(w http.ResponseWriter, r *http.Request) {
	m, ok := s.loadManifest(w)
	if !ok {
		return
	}
	_, err := os.Stat(filepath.Join(s.cfg.AppDir, "artisan"))
	writeJSON(w, map[string]any{
		"base_url":   s.baseURL(m),
		"can_rescan": s.cfg.AppDir != "" && err == nil,
	})
}

// --- Try it -----------------------------------------------------------------

type tryRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"` // "/api/users/1?x=y", always on the app's base URL
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type tryResponse struct {
	Status     int               `json:"status"`
	DurationMs float64           `json:"duration_ms"`
	SizeBytes  int               `json:"size_bytes"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Truncated  bool              `json:"truncated"`
}

const tryBodyLimit = 1 << 20

// POST /api/try sends one request to the app. The browser can't call the app
// directly (CORS), so the engine does it, but only on the app's own base URL.
func (s *srv) try(w http.ResponseWriter, r *http.Request) {
	var req tryRequest
	if !decode(w, r, &req) {
		return
	}
	m, ok := s.loadManifest(w)
	if !ok {
		return
	}

	switch req.Method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS":
	default:
		writeError(w, http.StatusBadRequest, "unsupported method "+req.Method)
		return
	}
	// A path only: no scheme or host, so this can't be pointed at other servers.
	if !strings.HasPrefix(req.Path, "/") || strings.HasPrefix(req.Path, "//") || strings.Contains(req.Path, "\\") {
		writeError(w, http.StatusBadRequest, "path must start with a single /")
		return
	}
	base := strings.TrimSuffix(s.baseURL(m), "/")
	if base == "" {
		writeError(w, http.StatusBadRequest, "no base URL: start serve with --base-url")
		return
	}
	target, err := url.Parse(base + req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path: "+err.Error())
		return
	}

	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}
	out, err := http.NewRequestWithContext(r.Context(), req.Method, target.String(), body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	for k, v := range req.Headers {
		out.Header.Set(k, v)
	}

	client := &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	start := time.Now()
	resp, err := client.Do(out)
	if err != nil {
		writeError(w, http.StatusBadGateway, "the app didn't answer: "+err.Error())
		return
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, tryBodyLimit+1))
	elapsed := time.Since(start)

	res := tryResponse{
		Status:     resp.StatusCode,
		DurationMs: float64(elapsed.Microseconds()) / 1000,
		SizeBytes:  len(data),
		Headers:    map[string]string{},
		Body:       string(data),
	}
	if len(data) > tryBodyLimit {
		res.Body, res.Truncated = string(data[:tryBodyLimit]), true
	}
	for k := range resp.Header {
		res.Headers[k] = resp.Header.Get(k)
	}
	writeJSON(w, res)
}

// --- Stress runs from the dashboard -------------------------------------------

type stressRequest struct {
	Routes      []string          `json:"routes"` // route ids (ids never contain *, so each matches only itself)
	Params      map[string]string `json:"params"`
	Token       string            `json:"token"`
	Concurrency int               `json:"concurrency"`
	DurationS   int               `json:"duration_s"`
	Requests    int               `json:"requests"`
}

// run is the one stress run the dashboard can have at a time.
type run struct {
	ID       string
	Started  time.Time
	Duration time.Duration
	Config   stress.Config
	Targets  []stress.Target
	BaseURL  string
	Manifest *manifest.Manifest
	Live     *stress.Live
	cancel   context.CancelFunc

	mu          sync.Mutex
	finished    time.Time
	done        bool
	interrupted bool
	report      string // file name in the reports dir, once saved
	err         string
}

type runner struct {
	mu  sync.Mutex
	cur *run
}

// POST /api/stress starts a run; 409 if one is already going.
func (s *srv) startStress(w http.ResponseWriter, r *http.Request) {
	var req stressRequest
	if !decode(w, r, &req) {
		return
	}
	m, ok := s.loadManifest(w)
	if !ok {
		return
	}
	if len(req.Routes) == 0 {
		writeError(w, http.StatusBadRequest, "pick at least one route")
		return
	}
	if req.Concurrency < 1 || req.Concurrency > 1000 {
		writeError(w, http.StatusBadRequest, "virtual users must be between 1 and 1000")
		return
	}
	if req.DurationS < 0 || req.DurationS > 3600 || req.Requests < 0 || (req.DurationS == 0 && req.Requests == 0) {
		writeError(w, http.StatusBadRequest, "set a duration (up to 1h) or a request count")
		return
	}

	header := http.Header{"Accept": {"application/json"}}
	if req.Token != "" {
		header.Set("Authorization", "Bearer "+req.Token)
	}
	plan, err := targets.Build(m, targets.Options{BaseURL: s.cfg.BaseURL, Routes: req.Routes, Params: req.Params, Header: header})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.runs.mu.Lock()
	defer s.runs.mu.Unlock()
	if cur := s.runs.cur; cur != nil && !cur.isDone() {
		writeError(w, http.StatusConflict, "a stress run is already in progress")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	run := &run{
		ID: fmt.Sprintf("%d", time.Now().UnixNano()), Started: time.Now(),
		Duration: time.Duration(req.DurationS) * time.Second,
		Config: stress.Config{Concurrency: req.Concurrency, Requests: req.Requests,
			Duration: time.Duration(req.DurationS) * time.Second, Timeout: 30 * time.Second},
		Targets: plan.Targets, BaseURL: plan.BaseURL, Manifest: m, Live: &stress.Live{}, cancel: cancel,
	}
	run.Config.Recorder = run.Live
	s.runs.cur = run

	go s.execute(ctx, run)
	writeJSON(w, s.snapshot(run))
}

func (s *srv) execute(ctx context.Context, run *run) {
	results, err := stress.Run(ctx, stress.NewClient(run.Config), run.Config, run.Targets)

	run.mu.Lock()
	defer run.mu.Unlock()
	run.finished = time.Now()
	run.done = true
	run.interrupted = ctx.Err() != nil
	if err != nil && !run.interrupted {
		run.err = err.Error()
		return
	}

	rep := report.NewStress(report.Run{
		EngineVersion: s.cfg.Version, Manifest: run.Manifest, BaseURL: run.BaseURL, Config: run.Config,
		StartedAt: run.Started, FinishedAt: run.finished, Interrupted: run.interrupted,
		Summary: stress.Summarize(run.Targets, results), Timeline: stress.Timeline(results, run.Started),
	})
	if err := os.MkdirAll(s.cfg.ReportsDir, 0o755); err != nil {
		run.err = "run finished but the report couldn't be saved: " + err.Error()
		return
	}
	name := "stress-" + run.Started.UTC().Format("20060102-150405") + ".json"
	if err := report.Write(filepath.Join(s.cfg.ReportsDir, name), rep); err != nil {
		run.err = "run finished but the report couldn't be saved: " + err.Error()
		return
	}
	run.report = name
}

func (r *run) isDone() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.done
}

// StressState is what GET /api/stress returns: a report-shaped live view.
type StressState struct {
	State       string         `json:"state"` // idle | running | done | failed
	ID          string         `json:"id,omitempty"`
	ElapsedMs   float64        `json:"elapsed_ms"`
	DurationMs  float64        `json:"duration_ms"`
	Interrupted bool           `json:"interrupted"`
	Report      string         `json:"report,omitempty"` // saved report file, when done
	Error       string         `json:"error,omitempty"`
	Live        *report.Stress `json:"live,omitempty"`
}

// GET /api/stress
func (s *srv) stressState(w http.ResponseWriter, r *http.Request) {
	s.runs.mu.Lock()
	cur := s.runs.cur
	s.runs.mu.Unlock()
	if cur == nil {
		writeJSON(w, StressState{State: "idle"})
		return
	}
	writeJSON(w, s.snapshot(cur))
}

func (s *srv) snapshot(run *run) StressState {
	results := run.Live.Results()

	run.mu.Lock()
	defer run.mu.Unlock()
	end := time.Now()
	state := "running"
	if run.done {
		end, state = run.finished, "done"
		if run.err != "" && run.report == "" {
			state = "failed"
		}
	}
	live := report.NewStress(report.Run{
		EngineVersion: s.cfg.Version, Manifest: run.Manifest, BaseURL: run.BaseURL, Config: run.Config,
		StartedAt: run.Started, FinishedAt: end, Interrupted: run.interrupted,
		Summary: stress.Summarize(run.Targets, results), Timeline: stress.Timeline(results, run.Started),
	})
	return StressState{
		State: state, ID: run.ID,
		ElapsedMs: float64(end.Sub(run.Started).Milliseconds()), DurationMs: float64(run.Duration.Milliseconds()),
		Interrupted: run.interrupted, Report: run.report, Error: run.err, Live: &live,
	}
}

// POST /api/stress/stop
func (s *srv) stopStress(w http.ResponseWriter, r *http.Request) {
	s.runs.mu.Lock()
	cur := s.runs.cur
	s.runs.mu.Unlock()
	if cur == nil || cur.isDone() {
		writeError(w, http.StatusConflict, "no stress run in progress")
		return
	}
	cur.cancel()
	writeJSON(w, map[string]bool{"stopping": true})
}

// --- Rescan -------------------------------------------------------------------

// POST /api/rescan runs `php artisan riven:export` in the app directory, so a
// route change shows up without leaving the dashboard.
func (s *srv) rescan(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AppDir == "" {
		writeError(w, http.StatusNotFound, "rescan is off (no app directory)")
		return
	}
	if _, err := os.Stat(filepath.Join(s.cfg.AppDir, "artisan")); err != nil {
		writeError(w, http.StatusNotFound, "no artisan file in "+s.cfg.AppDir+": run serve from your Laravel app")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "php", "artisan", "riven:export", "--no-ansi", "--output="+s.cfg.ManifestPath)
	cmd.Dir = s.cfg.AppDir
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		msg := err.Error()
		if errors.As(err, &exit) {
			msg = strings.TrimSpace(out.String())
		}
		writeError(w, http.StatusInternalServerError, "riven:export failed: "+msg)
		return
	}
	writeJSON(w, map[string]string{"output": strings.TrimSpace(out.String())})
}
