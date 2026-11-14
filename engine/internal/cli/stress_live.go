package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/adnan4k/riven/engine/server"
	"github.com/adnan4k/riven/engine/stress"
)

// liveWriter keeps a run's live state in a file, the way GET /api/stress
// shows it, and stops the run when the stop file appears. The Laravel-hosted
// dashboard starts `stress` in the background and polls that file, since a
// PHP request can't hold a running load test.
//
// All methods are safe on a nil *liveWriter (no --live-file), so callers
// don't need to check.
type liveWriter struct {
	path, stopPath string
	rec            *stress.Live
	done           chan struct{}
	wg             sync.WaitGroup

	mu  sync.Mutex
	run server.LiveRun
}

const liveInterval = 500 * time.Millisecond

func startLive(path, stopPath string, cancel func(), run server.LiveRun) *liveWriter {
	l := &liveWriter{path: path, stopPath: stopPath, rec: &stress.Live{}, done: make(chan struct{}), run: run}
	l.write()
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		tick := time.NewTicker(liveInterval)
		defer tick.Stop()
		for {
			select {
			case <-l.done:
				return
			case <-tick.C:
				if l.stopPath != "" {
					if _, err := os.Stat(l.stopPath); err == nil {
						cancel()
					}
				}
				l.write()
			}
		}
	}()
	return l
}

// stop ends the background writes (idempotent).
func (l *liveWriter) stop() {
	if l == nil {
		return
	}
	select {
	case <-l.done:
	default:
		close(l.done)
	}
	l.wg.Wait()
}

// finish writes the final state: done (with the saved report) or failed.
func (l *liveWriter) finish(at time.Time, interrupted bool, reportName, errMsg string) {
	if l == nil {
		return
	}
	l.stop()
	l.mu.Lock()
	l.run.Done, l.run.Finished, l.run.Interrupted, l.run.Report, l.run.Err = true, at, interrupted, reportName, errMsg
	l.mu.Unlock()
	l.write()
}

// write replaces the file atomically, so a reader never sees half of it.
func (l *liveWriter) write() {
	l.mu.Lock()
	state := server.NewStressState(l.run, l.rec.Results())
	l.mu.Unlock()

	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	tmp := l.path + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		os.Rename(tmp, l.path)
	}
}

// attachBodies gives each target the generated bodies for its route id.
func attachBodies(targets []stress.Target, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read --bodies-file: %w", err)
	}
	var byRoute map[string][]json.RawMessage
	if err := json.Unmarshal(data, &byRoute); err != nil {
		return fmt.Errorf("--bodies-file %s: %w", filepath.Base(path), err)
	}
	for i := range targets {
		t := &targets[i]
		for _, b := range byRoute[t.Name] {
			t.Bodies = append(t.Bodies, []byte(b))
		}
		if len(t.Bodies) > 0 {
			if t.Header == nil {
				t.Header = http.Header{}
			}
			t.Header.Set("Content-Type", "application/json")
		}
	}
	return nil
}
