package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// FakeResult is riven:fake-body's JSON output.
type FakeResult struct {
	RouteID     string              `json:"route_id"`
	FormRequest string              `json:"form_request"`
	Bodies      []json.RawMessage   `json:"bodies"`
	Invalid     int                 `json:"invalid"` // bodies that still fail the rules
	Errors      map[string][]string `json:"errors"`
	Warnings    []string            `json:"warnings"`
}

const maxFakeBodies = 500

// artisan runs `php artisan <args>` in the app directory.
func (s *srv) artisan(ctx context.Context, timeout time.Duration, args ...string) (stdout, stderr string, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "php", append([]string{"artisan"}, append(args, "--no-ansi")...)...)
	cmd.Dir = s.cfg.AppDir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err = cmd.Run()
	return out.String(), errb.String(), err
}

// fake generates request bodies for a route with riven:fake-body.
func (s *srv) fake(ctx context.Context, routeID string, count int) (*FakeResult, error) {
	if !s.canExport() {
		return nil, errors.New("generating test data needs the Laravel app: run serve from the app directory")
	}
	stdout, stderr, runErr := s.artisan(ctx, 2*time.Minute, "riven:fake-body", routeID, "--count="+strconv.Itoa(count))

	// The command prints one JSON object; anything before it (deprecation
	// notices…) is skipped.
	line := lastJSONLine(stdout)
	if line == "" {
		msg := strings.TrimSpace(stderr + "\n" + stdout)
		if runErr != nil && msg == "" {
			msg = runErr.Error()
		}
		return nil, fmt.Errorf("riven:fake-body failed: %s", msg)
	}

	var res struct {
		FakeResult
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(line), &res); err != nil {
		return nil, fmt.Errorf("riven:fake-body printed invalid JSON: %w", err)
	}
	if res.Error != "" {
		return nil, errors.New(res.Error)
	}
	return &res.FakeResult, nil
}

func lastJSONLine(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); strings.HasPrefix(l, "{") {
			return l
		}
	}
	return ""
}

// POST /api/fake {"route_id": "POST /api/users", "count": 1}
func (s *srv) fakeBody(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RouteID string `json:"route_id"`
		Count   int    `json:"count"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Count == 0 {
		req.Count = 1
	}
	if req.Count < 1 || req.Count > maxFakeBodies {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("count must be between 1 and %d", maxFakeBodies))
		return
	}
	res, err := s.fake(r.Context(), req.RouteID, req.Count)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, res)
}
