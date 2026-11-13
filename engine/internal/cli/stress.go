package cli

import (
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/faysal/riven/engine/internal/table"
	"github.com/faysal/riven/engine/manifest"
	"github.com/faysal/riven/engine/stress"
	"github.com/faysal/riven/engine/targets"
	"github.com/spf13/cobra"
)

type stressFlags struct {
	baseURL     string
	routes      []string
	params      []string
	headers     []string
	token       string
	concurrency int
	requests    int
	duration    time.Duration
	timeout     time.Duration
	dryRun      bool
}

func newStressCommand() *cobra.Command {
	var f stressFlags

	cmd := &cobra.Command{
		Use:   "stress",
		Short: "Send load to the app's routes",
		Long: `Send concurrent requests to routes from the manifest.

By default only GET routes are hit; other methods change data, so select
them explicitly with --route. Routes with required parameters need --param.`,
		Example: `  riven-engine stress -d 30s -c 20
  riven-engine stress --route "GET /api/users/*" --param user=1 --token $TOKEN
  riven-engine stress --route users.store -n 100 --dry-run`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runStress(cmd, f)
		},
	}

	fl := cmd.Flags()
	fl.StringVar(&f.baseURL, "base-url", "", "app URL to send requests to (default: app.url from the manifest)")
	fl.StringArrayVarP(&f.routes, "route", "r", nil, `route id or name to hit, * as wildcard (repeatable; default: all GET routes)`)
	fl.StringArrayVarP(&f.params, "param", "p", nil, "route parameter value as name=value (repeatable)")
	fl.StringArrayVarP(&f.headers, "header", "H", nil, `header as "Name: value" (repeatable)`)
	fl.StringVar(&f.token, "token", "", "bearer token, sent as Authorization: Bearer <token>")
	fl.IntVarP(&f.concurrency, "concurrency", "c", 10, "requests in flight at once")
	fl.IntVarP(&f.requests, "requests", "n", 0, "total requests to send (0 = no limit)")
	fl.DurationVarP(&f.duration, "duration", "d", 0, "how long to send requests, e.g. 30s (default 10s when -n isn't set)")
	fl.DurationVar(&f.timeout, "timeout", 30*time.Second, "per-request timeout")
	fl.BoolVar(&f.dryRun, "dry-run", false, "list the requests that would be sent, send nothing")

	return cmd
}

func runStress(cmd *cobra.Command, f stressFlags) error {
	path, _ := cmd.Flags().GetString("manifest")
	m, err := manifest.Load(path)
	if err != nil {
		return err
	}

	opts, err := f.targetOptions()
	if err != nil {
		return err
	}

	plan, err := targets.Build(m, opts)
	if err != nil {
		return err
	}

	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
	for _, s := range plan.Skipped {
		fmt.Fprintf(errOut, "skipped %s: %s\n", s.RouteID, s.Reason)
	}
	for _, w := range plan.Warnings {
		fmt.Fprintf(errOut, "warning: %s\n", w)
	}

	if f.dryRun {
		for _, t := range plan.Targets {
			fmt.Fprintf(out, "%-7s %s\n", t.Method, t.URL)
		}
		return nil
	}

	cfg := stress.Config{Concurrency: f.concurrency, Requests: f.requests, Duration: f.duration, Timeout: f.timeout}
	if cfg.Requests == 0 && cfg.Duration == 0 {
		cfg.Duration = 10 * time.Second
	}

	// Ctrl-C stops the run and still prints what was collected.
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()

	results, err := stress.Run(ctx, stress.NewClient(cfg), cfg, plan.Targets)
	if err != nil && ctx.Err() == nil {
		return err
	}
	if ctx.Err() != nil {
		fmt.Fprintln(errOut, "interrupted: showing results so far")
	}
	return printSummary(out, stress.Summarize(plan.Targets, results))
}

func (f stressFlags) targetOptions() (targets.Options, error) {
	opts := targets.Options{BaseURL: f.baseURL, Routes: f.routes, Params: map[string]string{}, Header: http.Header{}}

	for _, p := range f.params {
		name, value, ok := strings.Cut(p, "=")
		if !ok || name == "" {
			return opts, fmt.Errorf("--param %q must be name=value", p)
		}
		opts.Params[name] = value
	}
	for _, h := range f.headers {
		name, value, ok := strings.Cut(h, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return opts, fmt.Errorf(`--header %q must be "Name: value"`, h)
		}
		opts.Header.Add(strings.TrimSpace(name), strings.TrimSpace(value))
	}
	if f.token != "" {
		opts.Header.Set("Authorization", "Bearer "+f.token)
	}
	if opts.Header.Get("Accept") == "" {
		// Laravel answers errors with JSON (not an HTML page or a redirect) when asked to.
		opts.Header.Set("Accept", "application/json")
	}
	return opts, nil
}

func printSummary(w io.Writer, s stress.Summary) error {
	rows := make([][]string, 0, len(s.Targets)+1)
	for _, t := range s.Targets {
		rows = append(rows, statsRow(t.Name, t.Stats))
	}
	if len(s.Targets) > 1 {
		rows = append(rows, statsRow("Total", s.Total))
	}

	err := table.Render(w, []string{"Route", "Requests", "RPS", "p50", "p95", "p99", "Max", "Failed", "Statuses"}, rows)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "%d requests in %s, %.1f req/s\n", s.Total.Requests, s.Total.Elapsed.Round(time.Millisecond), s.Total.RPS)
	if len(s.Total.Errors) > 0 {
		fmt.Fprintf(w, "Transport errors: %s\n", counts(s.Total.Errors))
	}
	_, err = fmt.Fprintln(w, "Failed = transport errors + 5xx. Latency excludes transport errors.")
	return err
}

func statsRow(name string, st stress.Stats) []string {
	statuses := map[string]int{}
	for code, n := range st.Statuses {
		statuses[strconv.Itoa(code)] = n
	}
	if n := sum(st.Errors); n > 0 {
		statuses["error"] = n
	}

	return []string{
		name,
		strconv.Itoa(st.Requests),
		fmt.Sprintf("%.1f", st.RPS),
		ms(st.Latency.P50), ms(st.Latency.P95), ms(st.Latency.P99), ms(st.Latency.Max),
		fmt.Sprintf("%.1f%%", st.ErrorRate*100),
		counts(statuses),
	}
}

// counts formats {"200": 3, "500": 1} as "200×3 500×1", sorted by key.
func counts(m map[string]int) string {
	var parts []string
	for _, k := range slices.Sorted(maps.Keys(m)) {
		parts = append(parts, fmt.Sprintf("%s×%d", k, m[k]))
	}
	return strings.Join(parts, " ")
}

func sum(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

func ms(d time.Duration) string {
	switch {
	case d == 0:
		return "-"
	case d >= time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	default:
		return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
	}
}
