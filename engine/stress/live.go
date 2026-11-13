package stress

import (
	"sort"
	"sync"
	"time"
)

// Live collects results while a run is in progress so a UI can poll it.
// It is a Recorder: pass it as Config.Recorder.
type Live struct {
	mu      sync.Mutex
	results []Result
}

func (l *Live) Record(r Result) {
	l.mu.Lock()
	l.results = append(l.results, r)
	l.mu.Unlock()
}

// Results returns a copy of what has been recorded so far.
func (l *Live) Results() []Result {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Result(nil), l.results...)
}

// Point is one second of a run, for latency-over-time charts.
type Point struct {
	Second   int // seconds since the run started
	Requests int
	Failed   int // transport errors + 5xx, as in Stats.ErrorRate
	P50, P95 time.Duration
}

// Timeline buckets results by the second their request started in.
// Seconds with no requests are included (with zeros) so charts stay evenly spaced.
func Timeline(results []Result, start time.Time) []Point {
	if len(results) == 0 {
		return []Point{}
	}
	buckets := map[int][]Result{}
	last := 0
	for _, r := range results {
		sec := int(r.Start.Sub(start) / time.Second)
		if sec < 0 {
			sec = 0
		}
		buckets[sec] = append(buckets[sec], r)
		last = max(last, sec)
	}

	points := make([]Point, 0, last+1)
	for sec := 0; sec <= last; sec++ {
		p := Point{Second: sec}
		var lat []time.Duration
		for _, r := range buckets[sec] {
			p.Requests++
			if r.Err != nil || r.Status >= 500 {
				p.Failed++
			}
			if r.Err == nil {
				lat = append(lat, r.Latency)
			}
		}
		if len(lat) > 0 {
			sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
			p.P50, p.P95 = percentile(lat, 50), percentile(lat, 95)
		}
		points = append(points, p)
	}
	return points
}
