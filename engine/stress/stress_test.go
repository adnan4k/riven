package stress

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// server counts requests, peak concurrency and new TCP connections.
type server struct {
	*httptest.Server
	requests, inFlight, peak, conns atomic.Int64
}

func newServer(t *testing.T, delay time.Duration, status int) *server {
	t.Helper()
	s := &server{}
	s.Server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		now := s.inFlight.Add(1)
		for p := s.peak.Load(); now > p && !s.peak.CompareAndSwap(p, now); p = s.peak.Load() {
		}
		defer s.inFlight.Add(-1)

		io.Copy(io.Discard, r.Body)
		time.Sleep(delay)
		w.WriteHeader(status)
		w.Write([]byte(`{"data":[]}`))
	}))
	s.Config.ConnState = func(_ net.Conn, st http.ConnState) {
		if st == http.StateNew {
			s.conns.Add(1)
		}
	}
	s.Start()
	t.Cleanup(s.Close)
	return s
}

func targets(url string) []Target {
	return []Target{{Name: "GET /a", Method: "GET", URL: url + "/a"}, {Name: "POST /b", Method: "POST", URL: url + "/b", Body: []byte(`{}`)}}
}

func run(t *testing.T, cfg Config, ts []Target) []Result {
	t.Helper()
	results, err := Run(context.Background(), NewClient(cfg), cfg, ts)
	if err != nil {
		t.Fatal(err)
	}
	return results
}

func TestSendsExactlyTheRequestCount(t *testing.T) {
	s := newServer(t, time.Millisecond, http.StatusOK)

	results := run(t, Config{Concurrency: 8, Requests: 101}, targets(s.URL))

	if len(results) != 101 || s.requests.Load() != 101 {
		t.Fatalf("results = %d, server saw %d; want 101", len(results), s.requests.Load())
	}
	perTarget := map[int]int{}
	for _, r := range results {
		if r.Err != nil || r.Status != http.StatusOK || r.Latency <= 0 || r.Start.IsZero() {
			t.Fatalf("bad result %+v", r)
		}
		perTarget[r.Target]++
	}
	// Round-robin: 101 requests over 2 targets.
	if perTarget[0] != 51 || perTarget[1] != 50 {
		t.Errorf("per target = %v, want map[0:51 1:50]", perTarget)
	}
}

func TestNeverExceedsConcurrency(t *testing.T) {
	s := newServer(t, 5*time.Millisecond, http.StatusOK)

	run(t, Config{Concurrency: 4, Requests: 40}, targets(s.URL))

	if p := s.peak.Load(); p != 4 {
		t.Errorf("peak in-flight = %d, want exactly 4", p)
	}
}

func TestReusesConnections(t *testing.T) {
	s := newServer(t, time.Millisecond, http.StatusOK)

	run(t, Config{Concurrency: 10, Requests: 200}, targets(s.URL))

	// With the default transport (2 idle per host) this would be far above 10.
	if c := s.conns.Load(); c > 10 {
		t.Errorf("opened %d connections for 10 workers, want <= 10", c)
	}
}

func TestStopsAfterDuration(t *testing.T) {
	s := newServer(t, 10*time.Millisecond, http.StatusOK)

	start := time.Now()
	results := run(t, Config{Concurrency: 2, Duration: 100 * time.Millisecond}, targets(s.URL))
	elapsed := time.Since(start)

	// ~10 requests per worker; the last in-flight ones may finish just after the deadline.
	if elapsed < 100*time.Millisecond || elapsed > 300*time.Millisecond {
		t.Errorf("run took %v, want about 100ms", elapsed)
	}
	if len(results) < 10 || len(results) > 30 {
		t.Errorf("got %d results, want roughly 20", len(results))
	}
	for _, r := range results {
		if r.Err != nil {
			t.Fatalf("requests in flight at the deadline should complete, got %v", r.Err)
		}
	}
}

func TestRequestCountWinsOverLongDuration(t *testing.T) {
	s := newServer(t, 0, http.StatusOK)

	results := run(t, Config{Concurrency: 3, Requests: 7, Duration: time.Hour}, targets(s.URL))

	if len(results) != 7 {
		t.Errorf("got %d results, want 7", len(results))
	}
}

func TestNon2xxIsAResultNotAnError(t *testing.T) {
	s := newServer(t, 0, http.StatusUnprocessableEntity)

	results := run(t, Config{Concurrency: 1, Requests: 2}, targets(s.URL))

	for _, r := range results {
		if r.Err != nil || r.Status != http.StatusUnprocessableEntity {
			t.Errorf("result = %+v, want status 422 and no error", r)
		}
	}
}

func TestTimeoutsAndConnectionErrorsAreRecorded(t *testing.T) {
	slow := newServer(t, 200*time.Millisecond, http.StatusOK)

	results := run(t, Config{Concurrency: 1, Requests: 1, Timeout: 20 * time.Millisecond}, targets(slow.URL))
	if results[0].Err == nil || results[0].Status != 0 {
		t.Errorf("timeout result = %+v, want an error", results[0])
	}

	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	results = run(t, Config{Concurrency: 1, Requests: 1}, targets(dead.URL))
	if results[0].Err == nil {
		t.Errorf("closed server result = %+v, want a connection error", results[0])
	}
}

func TestCancelStopsAndDropsCutOffRequests(t *testing.T) {
	s := newServer(t, 50*time.Millisecond, http.StatusOK)
	cfg := Config{Concurrency: 4, Duration: time.Hour}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	results, err := Run(ctx, NewClient(cfg), cfg, targets(s.URL))

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want the context's error", err)
	}
	for _, r := range results {
		if r.Err != nil {
			t.Fatalf("cut-off request kept as a result: %+v", r)
		}
	}
}

func TestDoesNotFollowRedirects(t *testing.T) {
	s := httptest.NewServer(http.RedirectHandler("/elsewhere", http.StatusFound))
	defer s.Close()

	results := run(t, Config{Concurrency: 1, Requests: 1}, []Target{{Method: "GET", URL: s.URL}})

	if results[0].Status != http.StatusFound {
		t.Errorf("status = %d, want 302", results[0].Status)
	}
}

func TestSendsHeadersAndBody(t *testing.T) {
	var gotAuth, gotBody atomic.Value
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotAuth.Store(r.Header.Get("Authorization"))
		gotBody.Store(string(b))
	}))
	defer s.Close()

	run(t, Config{Concurrency: 1, Requests: 1}, []Target{{
		Method: "POST", URL: s.URL,
		Header: http.Header{"Authorization": {"Bearer t"}},
		Body:   []byte(`{"name":"x"}`),
	}})

	if gotAuth.Load() != "Bearer t" || gotBody.Load() != `{"name":"x"}` {
		t.Errorf("server got auth %q body %q", gotAuth.Load(), gotBody.Load())
	}
}

func TestRejectsBadConfig(t *testing.T) {
	ts := []Target{{Method: "GET", URL: "http://x"}}
	for name, tc := range map[string]struct {
		cfg Config
		ts  []Target
	}{
		"no targets": {Config{Concurrency: 1, Requests: 1}, nil},
		"no workers": {Config{Requests: 1}, ts},
		"no limit":   {Config{Concurrency: 1}, ts},
		"negative":   {Config{Concurrency: 1, Requests: -1}, ts},
	} {
		if _, err := Run(context.Background(), http.DefaultClient, tc.cfg, tc.ts); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
