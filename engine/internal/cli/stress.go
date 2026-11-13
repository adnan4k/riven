package cli

import (
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"time"

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

	start := time.Now()
	results, err := stress.Run(ctx, stress.NewClient(cfg), cfg, plan.Targets)
	if err != nil && ctx.Err() == nil {
		return err
	}
	printSummary(out, plan.Targets, results, time.Since(start))
	return nil
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

// printSummary is a placeholder until the metrics task: counts per target and status.
func printSummary(w io.Writer, ts []stress.Target, results []stress.Result, elapsed time.Duration) {
	type count struct {
		status map[int]int
		errors int
	}
	per := make([]count, len(ts))
	for i := range per {
		per[i].status = map[int]int{}
	}
	for _, r := range results {
		if r.Err != nil {
			per[r.Target].errors++
		} else {
			per[r.Target].status[r.Status]++
		}
	}

	fmt.Fprintf(w, "%d requests in %s\n", len(results), elapsed.Round(time.Millisecond))
	for i, t := range ts {
		var parts []string
		for _, s := range slices.Sorted(maps.Keys(per[i].status)) {
			parts = append(parts, fmt.Sprintf("%d×%d", s, per[i].status[s]))
		}
		if per[i].errors > 0 {
			parts = append(parts, fmt.Sprintf("errors×%d", per[i].errors))
		}
		fmt.Fprintf(w, "  %-30s %s\n", t.Name, strings.Join(parts, "  "))
	}
}
