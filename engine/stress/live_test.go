package stress

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestLiveRecordsEveryKeptResult(t *testing.T) {
	s := newServer(t, time.Millisecond, http.StatusOK)
	live := &Live{}
	cfg := Config{Concurrency: 4, Requests: 50, Recorder: live}

	results, err := Run(context.Background(), NewClient(cfg), cfg, targets(s.URL))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(live.Results()); got != len(results) || got != 50 {
		t.Errorf("live saw %d results, Run returned %d, want 50", got, len(results))
	}
}

func TestTimelineBucketsBySecond(t *testing.T) {
	results := []Result{
		res(0, 100, 10, 200),
		res(0, 900, 30, 200),
		res(0, 2500, 50, 500), // second 2; second 1 is empty
		{Target: 0, Start: t0.Add(2600 * time.Millisecond), Err: context.DeadlineExceeded},
	}

	pts := Timeline(results, t0)

	if len(pts) != 3 {
		t.Fatalf("got %d points, want 3 (empty second kept)", len(pts))
	}
	if pts[0].Requests != 2 || pts[0].P50 != ms(10) || pts[0].P95 != ms(30) {
		t.Errorf("second 0 = %+v", pts[0])
	}
	if pts[1].Requests != 0 || pts[1].P50 != 0 {
		t.Errorf("second 1 = %+v, want empty", pts[1])
	}
	if pts[2].Requests != 2 || pts[2].Failed != 2 || pts[2].P95 != ms(50) {
		t.Errorf("second 2 = %+v (500 and timeout both failed; timeout has no latency)", pts[2])
	}
	if len(Timeline(nil, t0)) != 0 {
		t.Error("no results => empty timeline")
	}
}
