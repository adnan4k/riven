package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStressDryRunOnSampleManifest(t *testing.T) {
	stdout, stderr, err := run(t, "stress", "-m", sampleManifest, "--dry-run")
	if err != nil {
		t.Fatal(err)
	}

	want := "GET     http://localhost:8000/api/users\nGET     http://localhost:8000/\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if !strings.Contains(stderr, "skipped GET /api/users/{user}: missing --param user") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestStressDryRunWithParamsAndExplicitRoutes(t *testing.T) {
	stdout, stderr, err := run(t, "stress", "-m", sampleManifest, "--dry-run",
		"--base-url", "http://127.0.0.1:9000", "-r", "users.*", "-p", "user=7")
	if err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{
		"GET     http://127.0.0.1:9000/api/users\n",
		"POST    http://127.0.0.1:9000/api/users\n",
		"PUT     http://127.0.0.1:9000/api/users/7\n",
		"DELETE  http://127.0.0.1:9000/api/users/7\n",
	} {
		if !strings.Contains(stdout, line) {
			t.Errorf("stdout missing %q:\n%s", line, stdout)
		}
	}
	if !strings.Contains(stderr, "warning: POST /api/users: validates a request body") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestStressSendsLoadWithHeaders(t *testing.T) {
	var hits, authed atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Authorization") == "Bearer secret" && r.Header.Get("Accept") == "application/json" && r.Header.Get("X-Team") == "a" {
			authed.Add(1)
		}
	}))
	defer srv.Close()

	stdout, _, err := run(t, "stress", "-m", sampleManifest, "--base-url", srv.URL,
		"-n", "20", "-c", "4", "--token", "secret", "-H", "X-Team: a")
	if err != nil {
		t.Fatal(err)
	}

	if hits.Load() != 20 || authed.Load() != 20 {
		t.Errorf("hits = %d, with headers = %d; want 20, 20", hits.Load(), authed.Load())
	}
	for _, want := range []string{"| GET /api/users | 10       |", "| Total          | 20       |", "200×20", "| 0.0%   |", "20 requests in "} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestStressRejectsBadFlags(t *testing.T) {
	for _, args := range [][]string{
		{"-p", "user"},
		{"-H", "no-colon"},
		{"-r", "GET /nope"},
	} {
		_, _, err := run(t, append([]string{"stress", "-m", sampleManifest, "--dry-run"}, args...)...)
		if err == nil {
			t.Errorf("%v: want an error", args)
		}
	}
}

func TestStressSummaryCountsFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	stdout, _, err := run(t, "stress", "-m", sampleManifest, "--base-url", srv.URL, "-n", "10", "-c", "2")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"| GET /          | 5        |", "| 100.0% | 500×5", "| 50.0%  | 200×5 500×5"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestFormatMs(t *testing.T) {
	for d, want := range map[time.Duration]string{0: "-", 420 * time.Microsecond: "0.4ms", 12345 * time.Microsecond: "12.3ms", 1500 * time.Millisecond: "1.50s"} {
		if got := ms(d); got != want {
			t.Errorf("ms(%v) = %q, want %q", d, got, want)
		}
	}
}
