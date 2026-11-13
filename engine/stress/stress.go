// Package stress sends HTTP load with a fixed-size pool of goroutines and
// records one Result per request. It knows nothing about the manifest:
// callers turn routes into Targets.
package stress

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Target is one request to send. Workers cycle through targets in order.
type Target struct {
	Name   string // label for reports, e.g. the route id "GET /api/users"
	Method string
	URL    string
	Header http.Header
	Body   []byte
}

// Config controls the load. At least one of Requests or Duration must be set;
// with both, the run stops at whichever limit is hit first.
type Config struct {
	Concurrency int           // number of workers, i.e. max requests in flight
	Requests    int           // total requests to send; 0 = no limit
	Duration    time.Duration // stop starting new requests after this; 0 = no limit
	Timeout     time.Duration // per-request timeout; 0 = 30s
}

// Result is the outcome of one request.
type Result struct {
	Target  int           // index into the targets slice
	Start   time.Time     // when the request was sent
	Latency time.Duration // until the full response body was read
	Status  int           // HTTP status, 0 when Err is set
	Err     error         // transport error or timeout; non-2xx is not an error here
}

func (c Config) validate(targets []Target) error {
	switch {
	case len(targets) == 0:
		return errors.New("no targets to send requests to")
	case c.Concurrency < 1:
		return fmt.Errorf("concurrency must be at least 1, got %d", c.Concurrency)
	case c.Requests < 0 || c.Duration < 0:
		return errors.New("requests and duration can't be negative")
	case c.Requests == 0 && c.Duration == 0:
		return errors.New("set a request count, a duration, or both")
	}
	return nil
}

// NewClient returns an HTTP client sized for the pool. Go's default keeps
// only 2 idle connections per host, so with more workers most requests would
// open a new TCP connection: you'd measure handshakes, not the app, and can
// run out of local ports.
func NewClient(cfg Config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = cfg.Concurrency
	transport.MaxIdleConnsPerHost = cfg.Concurrency

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		// Measure the route itself; a redirect is a valid 3xx answer.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// Run sends load until a limit is hit or ctx is cancelled. Requests already
// in flight when the duration ends are allowed to finish. Requests cut off by
// cancelling ctx (e.g. Ctrl-C) are dropped, since they say nothing about the app.
func Run(ctx context.Context, client *http.Client, cfg Config, targets []Target) ([]Result, error) {
	if err := cfg.validate(targets); err != nil {
		return nil, err
	}

	// issuing only controls when to stop starting requests; ctx governs the requests themselves.
	issuing := ctx
	if cfg.Duration > 0 {
		var cancel context.CancelFunc
		issuing, cancel = context.WithTimeout(ctx, cfg.Duration)
		defer cancel()
	}

	var (
		issued  atomic.Int64
		wg      sync.WaitGroup
		perWork = make([][]Result, cfg.Concurrency)
	)

	for w := range cfg.Concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for issuing.Err() == nil {
				n := issued.Add(1) - 1 // 0-based request number, unique across workers
				if cfg.Requests > 0 && n >= int64(cfg.Requests) {
					return
				}

				i := int(n % int64(len(targets)))
				r := send(ctx, client, targets[i])
				if ctx.Err() != nil {
					return
				}
				r.Target = i
				// Each worker appends to its own slice: no lock, no channel.
				perWork[w] = append(perWork[w], r)
			}
		}()
	}
	wg.Wait()

	var results []Result
	for _, rs := range perWork {
		results = append(results, rs...)
	}

	return results, ctx.Err()
}

func send(ctx context.Context, client *http.Client, t Target) Result {
	var body io.Reader
	if t.Body != nil {
		body = bytes.NewReader(t.Body)
	}

	req, err := http.NewRequestWithContext(ctx, t.Method, t.URL, body)
	if err != nil {
		return Result{Start: time.Now(), Err: err}
	}
	for k, vs := range t.Header {
		req.Header[k] = vs
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return Result{Start: start, Latency: time.Since(start), Err: err}
	}
	// Read the whole body: latency should include it, and an unread body
	// stops the connection from being reused.
	_, err = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	return Result{Start: start, Latency: time.Since(start), Status: resp.StatusCode, Err: err}
}
