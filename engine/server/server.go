// Package server serves the dashboard and the JSON API it reads:
//
//	GET /api/manifest        the manifest, re-read on every request
//	GET /api/openapi         the OpenAPI document built from it
//	GET /api/reports         stress report summaries, newest first
//	GET /api/reports/{name}  one stress report file
//	GET /api/config          base URL, whether rescan is available
//	POST /api/try            send one request to the app (Try it)
//	GET|POST /api/stress     live state of / start a stress run; POST /api/stress/stop
//	POST /api/rescan         run php artisan riven:export
//	GET /api/events          Server-Sent Events: manifest changed, scan running/done/failed
//
// Every other path serves the embedded dashboard (index.html for unknown
// paths, so client-side routes work on reload).
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/faysal/riven/engine/manifest"
	"github.com/faysal/riven/engine/openapi"
	"github.com/faysal/riven/engine/report"
)

type Config struct {
	ManifestPath string
	ReportsDir   string
	UI           fs.FS  // built dashboard; without index.html a build hint is shown
	BaseURL      string // app URL for Try it and stress runs; default: the manifest's app.url
	AppDir       string // Laravel app root, for Rescan (php artisan riven:export); "" disables it
	Version      string // engine version, written into saved reports
	AllowAnyHost bool   // serving on a LAN address on purpose: skip the localhost Host check
	AutoExport   bool   // Watch re-runs riven:export when the app's PHP files change
}

// Server is the dashboard's HTTP handler. Files are read per request, so a
// new export or stress report shows up on the next request; Watch also pushes
// changes to open dashboards.
type Server struct {
	http.Handler
	s *srv
}

// Watch syncs with the app until ctx is done (see srv.Watch).
func (sv *Server) Watch(ctx context.Context) { sv.s.Watch(ctx) }

// Close ends open event streams; call it when shutting down.
func (sv *Server) Close() { sv.s.hub.close() }

func New(cfg Config) *Server {
	s := &srv{cfg: cfg}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", s.config)
	mux.HandleFunc("POST /api/try", s.guard(s.try))
	mux.HandleFunc("GET /api/stress", s.stressState)
	mux.HandleFunc("POST /api/stress", s.guard(s.startStress))
	mux.HandleFunc("POST /api/stress/stop", s.guard(s.stopStress))
	mux.HandleFunc("POST /api/rescan", s.guard(s.rescan))
	mux.HandleFunc("GET /api/events", s.events)
	mux.HandleFunc("GET /api/manifest", s.manifest)
	mux.HandleFunc("GET /api/openapi", s.openapi)
	mux.HandleFunc("GET /api/reports", s.reports)
	mux.HandleFunc("GET /api/reports/{name}", s.report)
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "no such API endpoint")
	})
	mux.HandleFunc("GET /", s.ui)
	return &Server{Handler: mux, s: s}
}

type srv struct {
	cfg      Config
	runs     runner
	hub      hub
	exportMu sync.Mutex
}

func (s *srv) loadManifest(w http.ResponseWriter) (*manifest.Manifest, bool) {
	m, err := manifest.Load(s.cfg.ManifestPath)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return nil, false
	}
	return m, true
}

func (s *srv) manifest(w http.ResponseWriter, r *http.Request) {
	if m, ok := s.loadManifest(w); ok {
		writeJSON(w, m)
	}
}

func (s *srv) openapi(w http.ResponseWriter, r *http.Request) {
	if m, ok := s.loadManifest(w); ok {
		writeJSON(w, openapi.Build(m, openapi.Options{}))
	}
}

// ReportSummary is one entry of GET /api/reports.
type ReportSummary struct {
	Name        string        `json:"name"` // file name, for GET /api/reports/{name}
	StartedAt   time.Time     `json:"started_at"`
	Interrupted bool          `json:"interrupted"`
	Config      report.Config `json:"config"`
	Summary     report.Stats  `json:"summary"`
	Routes      int           `json:"routes"`
}

func (s *srv) reports(w http.ResponseWriter, r *http.Request) {
	list := []ReportSummary{}

	root, err := os.OpenRoot(s.cfg.ReportsDir)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, list) // no reports yet is normal
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer root.Close()

	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		rep, err := readReport(root, e.Name())
		if err != nil {
			continue // not a stress report (or another tool's JSON): skip it
		}
		list = append(list, ReportSummary{
			Name: e.Name(), StartedAt: rep.StartedAt, Interrupted: rep.Interrupted,
			Config: rep.Config, Summary: rep.Summary, Routes: len(rep.Routes),
		})
	}
	slices.SortFunc(list, func(a, b ReportSummary) int { return b.StartedAt.Compare(a.StartedAt) })
	writeJSON(w, list)
}

func (s *srv) report(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !strings.HasSuffix(name, ".json") {
		writeError(w, http.StatusNotFound, "reports are .json files")
		return
	}

	// os.Root refuses names that escape the directory ("../x", symlinks out).
	root, err := os.OpenRoot(s.cfg.ReportsDir)
	if err != nil {
		writeError(w, http.StatusNotFound, "no reports directory")
		return
	}
	defer root.Close()

	rep, err := readReport(root, name)
	if err != nil {
		writeError(w, http.StatusNotFound, "no stress report named "+name)
		return
	}
	writeJSON(w, rep)
}

func readReport(root *os.Root, name string) (*report.Stress, error) {
	data, err := root.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var rep report.Stress
	if err := json.Unmarshal(data, &rep); err != nil {
		return nil, err
	}
	if rep.Kind != "stress" || rep.ReportVersion != report.Version {
		return nil, errors.New("not a supported stress report")
	}
	return &rep, nil
}

func (s *srv) ui(w http.ResponseWriter, r *http.Request) {
	if _, err := fs.Stat(s.cfg.UI, "index.html"); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(notBuilt))
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if st, err := fs.Stat(s.cfg.UI, path); err != nil || st.IsDir() {
		// Client-side route (e.g. /routes/GET%20%2Fapi%2Fusers): let React handle it.
		path = "index.html"
	}
	if path != "index.html" && strings.HasPrefix(path, "assets/") {
		// Vite puts a content hash in asset names, so they can be cached for good.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeFileFS(w, r, s.cfg.UI, path)
}

const notBuilt = `<!doctype html><meta charset="utf-8"><title>Riven</title>
<body style="font-family:system-ui;max-width:40rem;margin:4rem auto;line-height:1.5">
<h1>The dashboard isn't built into this binary</h1>
<p>Build it, then rebuild riven-engine:</p>
<pre>cd engine/web && npm install && npm run build
cd .. && go build ./cmd/riven-engine</pre>
<p>The API works already: <a href="/api/manifest">/api/manifest</a>, <a href="/api/openapi">/api/openapi</a>, <a href="/api/reports">/api/reports</a>.</p>`

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
