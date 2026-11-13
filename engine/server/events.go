package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/adnan4k/riven/engine/watch"
)

// Event is pushed to open dashboards over Server-Sent Events.
type Event struct {
	Type string `json:"type"` // "manifest" (re-read it) | "scan" (an export ran)
	// For "scan": running | done | failed, and the export's output or error.
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`
}

// hub fans events out to every connected dashboard.
type hub struct {
	mu     sync.Mutex
	subs   map[chan Event]struct{}
	closed bool
}

func (h *hub) subscribe() chan Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan Event, 16)
	if h.closed {
		close(ch)
		return ch
	}
	if h.subs == nil {
		h.subs = map[chan Event]struct{}{}
	}
	h.subs[ch] = struct{}{}
	return ch
}

func (h *hub) unsubscribe(ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
}

// publish never blocks: a dashboard too slow to read misses events, and the
// next "manifest" event makes it reload everything anyway.
func (h *hub) publish(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// close ends every stream, so server shutdown doesn't wait on open dashboards.
func (h *hub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for ch := range h.subs {
		close(ch)
	}
	h.subs = nil
}

// GET /api/events streams Events as Server-Sent Events.
func (s *srv) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := s.hub.subscribe()
	defer s.hub.unsubscribe(ch)

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(25 * time.Second) // keeps proxies from closing an idle stream
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n")
		case e, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data)
		}
		flusher.Flush()
	}
}

// Watch keeps the dashboard in sync with the app until ctx is done:
//   - the manifest file changing (any export, also one run by hand) is pushed
//     to dashboards as a "manifest" event;
//   - with AutoExport, PHP changes under routes/, app/, config/ and bootstrap/
//     re-run php artisan riven:export, so new routes show up on their own.
func (s *srv) Watch(ctx context.Context) {
	dir, file := filepath.Split(s.cfg.ManifestPath)
	go watch.Watcher{
		Root: dir, Files: []string{file}, Interval: 500 * time.Millisecond, Quiet: 200 * time.Millisecond,
	}.Run(ctx, func() { s.hub.publish(Event{Type: "manifest"}) })

	if s.cfg.AutoExport && s.canExport() {
		go watch.Watcher{
			Root:     s.cfg.AppDir,
			Dirs:     []string{"routes", "app", "config", "bootstrap"},
			Ext:      ".php",
			Skip:     []string{"vendor", "node_modules", "storage", "bootstrap/cache"},
			Interval: time.Second,
			Quiet:    500 * time.Millisecond,
		}.Run(ctx, func() { s.export(ctx) })
	}
}

func (s *srv) canExport() bool {
	if s.cfg.AppDir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(s.cfg.AppDir, "artisan"))
	return err == nil
}
