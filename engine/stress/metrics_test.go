package stress

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

// res makes a result that starts at t0+startMs and takes latencyMs.
func res(target, startMs, latencyMs, status int) Result {
	return Result{Target: target, Start: t0.Add(ms(startMs)), Latency: ms(latencyMs), Status: status}
}

func TestPercentilesNearestRank(t *testing.T) {
	var ds []time.Duration
	for i := 100; i >= 1; i-- { // unsorted on purpose
		ds = append(ds, ms(i))
	}

	l := latency(ds)

	want := Latency{Min: ms(1), Mean: ms(50) + 500*time.Microsecond, P50: ms(50), P90: ms(90), P95: ms(95), P99: ms(99), Max: ms(100)}
	if l != want {
		t.Errorf("latency = %+v\nwant      %+v", l, want)
	}
}

func TestPercentilesOfFewSamplesAreRealSamples(t *testing.T) {
	l := latency([]time.Duration{ms(10), ms(20), ms(30)})

	if l.P50 != ms(20) || l.P95 != ms(30) || l.P99 != ms(30) {
		t.Errorf("latency = %+v", l)
	}
	if one := latency([]time.Duration{ms(7)}); one.P50 != ms(7) || one.P99 != ms(7) {
		t.Errorf("single sample = %+v", one)
	}
}

func TestSummarizePerTargetAndTotal(t *testing.T) {
	targets := []Target{{Name: "GET /a", Method: "GET", URL: "http://x/a"}, {Name: "POST /b", Method: "POST", URL: "http://x/b"}}
	results := []Result{
		res(0, 0, 100, 200),
		res(0, 100, 100, 200),
		res(0, 200, 300, 500),
		res(1, 0, 50, 422),
		{Target: 1, Start: t0.Add(ms(400)), Latency: ms(600), Err: context.DeadlineExceeded},
	}

	s := Summarize(targets, results)

	a, b := s.Targets[0], s.Targets[1]
	if a.Name != "GET /a" || a.Requests != 3 || a.Elapsed != ms(500) || a.RPS != 6 {
		t.Errorf("a = %+v", a)
	}
	if a.Statuses[200] != 2 || a.Statuses[500] != 1 || fmt.Sprintf("%.3f", a.ErrorRate) != "0.333" {
		t.Errorf("a statuses/error rate = %v / %v", a.Statuses, a.ErrorRate)
	}
	if a.Latency.P50 != ms(100) || a.Latency.Max != ms(300) {
		t.Errorf("a latency = %+v", a.Latency)
	}

	// The timeout counts as a failure but not as a latency sample; 422 isn't a failure.
	if b.Errors["timeout"] != 1 || b.ErrorRate != 0.5 || b.Latency.Max != ms(50) || b.Statuses[422] != 1 {
		t.Errorf("b = %+v", b.Stats)
	}

	if s.Total.Requests != 5 || s.Total.Elapsed != ms(1000) || s.Total.RPS != 5 || s.Total.ErrorRate != 0.4 {
		t.Errorf("total = %+v", s.Total)
	}
}

func TestSummarizeTargetWithNoResults(t *testing.T) {
	s := Summarize([]Target{{Name: "GET /a"}}, nil)

	if s.Targets[0].Requests != 0 || s.Targets[0].RPS != 0 || s.Total.ErrorRate != 0 {
		t.Errorf("empty summary = %+v", s)
	}
}

func TestErrorKinds(t *testing.T) {
	for err, want := range map[error]string{
		context.DeadlineExceeded:                     "timeout",
		fmt.Errorf("dial: %w", syscall.ECONNREFUSED): "connection refused",
		fmt.Errorf("read: %w", syscall.ECONNRESET):   "connection reset",
		errors.New("something else"):                 "other",
	} {
		if got := errorKind(err); got != want {
			t.Errorf("errorKind(%v) = %q, want %q", err, got, want)
		}
	}
}

func TestErrorKindsFromRealFailures(t *testing.T) {
	slow := newServer(t, 200*time.Millisecond, http.StatusOK)
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()

	for url, want := range map[string]string{slow.URL: "timeout", dead.URL: "connection refused"} {
		cfg := Config{Concurrency: 1, Requests: 1, Timeout: 20 * time.Millisecond}
		ts := []Target{{Method: "GET", URL: url}}
		results, err := Run(context.Background(), NewClient(cfg), cfg, ts)
		if err != nil {
			t.Fatal(err)
		}
		if got := Summarize(ts, results).Total.Errors; got[want] != 1 {
			t.Errorf("%s: errors = %v, want %s", url, got, want)
		}
	}
}
