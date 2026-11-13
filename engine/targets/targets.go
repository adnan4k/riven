// Package targets turns manifest routes into stress.Targets: it picks the
// routes, fills in route parameters and adds headers.
package targets

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/adnan4k/riven/engine/manifest"
	"github.com/adnan4k/riven/engine/stress"
)

type Options struct {
	BaseURL string // scheme://host[:port], defaults to the manifest's app.url
	// Routes selects routes by id ("GET /api/users") or name ("users.index"),
	// with * matching anything. Empty means every GET route: other methods
	// change data, so they're only sent when selected explicitly.
	Routes []string
	Params map[string]string // route parameter values, e.g. user=1
	Header http.Header       // sent with every request, e.g. Authorization
}

// Skipped is a selected route that can't be sent as-is.
type Skipped struct {
	RouteID string
	Reason  string
}

type Plan struct {
	BaseURL  string // the one actually used: the option or the manifest's app.url
	Targets  []stress.Target
	Skipped  []Skipped
	Warnings []string // targets that will be sent but probably fail (401, 422)
}

var paramPattern = regexp.MustCompile(`\{(\w+)(\?)?\}`)

// Build selects routes and turns them into targets.
func Build(m *manifest.Manifest, opts Options) (Plan, error) {
	base, err := baseURL(m, opts.BaseURL)
	if err != nil {
		return Plan{}, err
	}

	routes, err := selectRoutes(m.Routes, opts.Routes)
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{BaseURL: base.String()}
	for _, r := range routes {
		u, err := routeURL(base, r, opts.Params)
		if err != nil {
			plan.Skipped = append(plan.Skipped, Skipped{RouteID: r.ID, Reason: err.Error()})
			continue
		}

		method := method(r)
		plan.Targets = append(plan.Targets, stress.Target{Name: r.ID, Method: method, URL: u, Header: opts.Header.Clone()})
		plan.Warnings = append(plan.Warnings, warnings(r, method, opts.Header)...)
	}

	if len(plan.Targets) == 0 {
		return plan, errors.New("no routes can be sent" + skippedSummary(plan.Skipped))
	}

	return plan, nil
}

func baseURL(m *manifest.Manifest, flag string) (*url.URL, error) {
	raw := flag
	if raw == "" && m.App.URL != nil {
		raw = *m.App.URL
	}
	if raw == "" {
		return nil, errors.New("no base URL: pass --base-url (the manifest has no app.url)")
	}

	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("base URL %q must look like http://localhost:8000", raw)
	}
	return u, nil
}

func selectRoutes(routes []manifest.Route, patterns []string) ([]manifest.Route, error) {
	if len(patterns) == 0 {
		var gets []manifest.Route
		for _, r := range routes {
			if method(r) == http.MethodGet {
				gets = append(gets, r)
			}
		}
		if len(gets) == 0 {
			return nil, errors.New("the manifest has no GET routes; select routes with --route")
		}
		return gets, nil
	}

	var selected []manifest.Route
	for _, p := range patterns {
		re := glob(p)
		found := false
		for _, r := range routes {
			if re.MatchString(r.ID) || (r.Name != nil && re.MatchString(*r.Name)) {
				found = true
				if !slices.ContainsFunc(selected, func(s manifest.Route) bool { return s.ID == r.ID }) {
					selected = append(selected, r)
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("--route %q matches no route id or name (see `riven-engine routes`)", p)
		}
	}
	return selected, nil
}

// glob compiles a pattern where * matches anything (like Laravel's Str::is).
func glob(p string) *regexp.Regexp {
	parts := strings.Split(p, "*")
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	return regexp.MustCompile("^" + strings.Join(parts, ".*") + "$")
}

// method is the route's main method: the one in its id (first non-HEAD).
func method(r manifest.Route) string {
	m, _, _ := strings.Cut(r.ID, " ")
	return m
}

func routeURL(base *url.URL, r manifest.Route, values map[string]string) (string, error) {
	patterns := map[string]*string{}
	for _, p := range r.Parameters {
		patterns[p.Name] = p.Pattern
	}

	var missing []string
	fill := func(s string, escape func(string) string) (string, error) {
		var ferr error
		out := paramPattern.ReplaceAllStringFunc(s, func(m string) string {
			sub := paramPattern.FindStringSubmatch(m)
			name, optional := sub[1], sub[2] == "?"
			v, ok := values[name]
			if !ok {
				if !optional {
					missing = append(missing, name)
				}
				return ""
			}
			if err := checkPattern(name, v, patterns[name]); err != nil && ferr == nil {
				ferr = err
			}
			return escape(v)
		})
		return out, ferr
	}

	u := *base
	if r.Domain != nil {
		host, err := fill(*r.Domain, func(v string) string { return v })
		if err != nil {
			return "", err
		}
		// Keep the base URL's port: the domain route has none of its own.
		if port := base.Port(); port != "" {
			host += ":" + port
		}
		u.Host = host
	}

	path, err := fill(r.URI, url.PathEscape)
	if err != nil {
		return "", err
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("missing --param %s", strings.Join(missing, ", --param "))
	}

	// Dropped optional parameters leave "a//b" or a trailing slash, which Laravel wouldn't route.
	path = strings.TrimSuffix(regexp.MustCompile(`/{2,}`).ReplaceAllString("/"+path, "/"), "/")
	escaped := strings.TrimSuffix(base.EscapedPath(), "/") + path
	if escaped == "" {
		escaped = "/"
	}
	// path holds escaped values ("a%2Fb"); setting it as Path would escape it again.
	parsed, err := url.Parse(escaped)
	if err != nil {
		return "", err
	}
	u.Path, u.RawPath = parsed.Path, parsed.RawPath
	return u.String(), nil
}

// checkPattern applies the route's where() constraint, which Laravel checks
// as a full match. PCRE-only syntax that Go can't compile is skipped.
func checkPattern(name, value string, pattern *string) error {
	if pattern == nil {
		return nil
	}
	re, err := regexp.Compile(`^(?:` + *pattern + `)$`)
	if err != nil || re.MatchString(value) {
		return nil
	}
	return fmt.Errorf("--param %s=%s doesn't match the route's pattern %s", name, value, *pattern)
}

func warnings(r manifest.Route, method string, h http.Header) []string {
	var out []string
	if r.RequiresAuth() && h.Get("Authorization") == "" && h.Get("Cookie") == "" {
		out = append(out, r.ID+": requires authentication but no --token or --header was given; expect 401")
	}
	// nil rules mean "unknown" (rules() needed a live request), so they still validate; only {} is safe.
	hasRules := r.Request.Rules == nil || len(r.Request.Rules) > 0
	if method != http.MethodGet && method != http.MethodDelete && r.Request.FormRequest != nil && hasRules {
		out = append(out, r.ID+": validates a request body, which isn't sent yet; expect 422")
	}
	return out
}

func skippedSummary(skipped []Skipped) string {
	var b strings.Builder
	for _, s := range skipped {
		fmt.Fprintf(&b, "\n  %s: %s", s.RouteID, s.Reason)
	}
	return b.String()
}
