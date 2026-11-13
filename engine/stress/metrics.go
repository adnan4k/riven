package stress

import (
	"context"
	"errors"
	"math"
	"net"
	"slices"
	"syscall"
	"time"
)

// Summary aggregates results per target and overall.
type Summary struct {
	Total   Stats
	Targets []TargetStats // same order as the targets passed to Run
}

type TargetStats struct {
	Name   string
	Method string
	URL    string
	Stats
}

type Stats struct {
	Requests int
	// Elapsed runs from the first request's start to the last response's end.
	Elapsed time.Duration
	RPS     float64
	// Latency covers requests that got a response (any status). Transport
	// errors are left out: a timeout's "latency" is just the timeout setting.
	Latency  Latency
	Statuses map[int]int    // status code => count
	Errors   map[string]int // transport error kind => count (see errorKind)
	// ErrorRate is (transport errors + 5xx) / Requests. 4xx are counted in
	// Statuses but not here: 401/422 are often expected (no token, no body).
	ErrorRate float64
}

type Latency struct {
	Min, Mean, P50, P90, P95, P99, Max time.Duration
}

// Summarize computes stats from Run's results.
func Summarize(targets []Target, results []Result) Summary {
	per := make([][]Result, len(targets))
	for _, r := range results {
		per[r.Target] = append(per[r.Target], r)
	}

	s := Summary{Total: stats(results)}
	for i, t := range targets {
		s.Targets = append(s.Targets, TargetStats{Name: t.Name, Method: t.Method, URL: t.URL, Stats: stats(per[i])})
	}
	return s
}

func stats(results []Result) Stats {
	st := Stats{Requests: len(results), Statuses: map[int]int{}, Errors: map[string]int{}}
	if len(results) == 0 {
		return st
	}

	var (
		latencies   []time.Duration
		failed      int
		first, last time.Time
	)
	for i, r := range results {
		end := r.Start.Add(r.Latency)
		if i == 0 || r.Start.Before(first) {
			first = r.Start
		}
		if end.After(last) {
			last = end
		}

		if r.Err != nil {
			st.Errors[errorKind(r.Err)]++
			failed++
			continue
		}
		st.Statuses[r.Status]++
		if r.Status >= 500 {
			failed++
		}
		latencies = append(latencies, r.Latency)
	}

	st.Elapsed = last.Sub(first)
	if st.Elapsed > 0 {
		st.RPS = float64(st.Requests) / st.Elapsed.Seconds()
	}
	st.ErrorRate = float64(failed) / float64(st.Requests)
	st.Latency = latency(latencies)
	return st
}

func latency(ds []time.Duration) Latency {
	if len(ds) == 0 {
		return Latency{}
	}
	slices.Sort(ds)

	var sum time.Duration
	for _, d := range ds {
		sum += d
	}

	return Latency{
		Min:  ds[0],
		Mean: sum / time.Duration(len(ds)),
		P50:  percentile(ds, 50),
		P90:  percentile(ds, 90),
		P95:  percentile(ds, 95),
		P99:  percentile(ds, 99),
		Max:  ds[len(ds)-1],
	}
}

// percentile uses the nearest-rank method on sorted values: the smallest
// value with at least p% of samples at or below it. It always returns a
// real sample (no interpolation), so p99 of 10 requests is the slowest one.
func percentile(sorted []time.Duration, p float64) time.Duration {
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	return sorted[max(rank, 1)-1]
}

// errorKind groups transport errors so a report shows "timeout ×40", not 40
// slightly different messages.
func errorKind(err error) string {
	var netErr net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return "timeout"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused"
	case errors.Is(err, syscall.ECONNRESET), errors.Is(err, syscall.EPIPE):
		return "connection reset"
	default:
		return "other"
	}
}
