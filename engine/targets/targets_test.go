package targets

import (
	"net/http"
	"strings"
	"testing"

	"github.com/adnan4k/riven/engine/manifest"
)

func ptr[T any](v T) *T { return &v }

func route(id string, opts ...func(*manifest.Route)) manifest.Route {
	_, uri, _ := strings.Cut(id, " ")
	r := manifest.Route{ID: id, URI: strings.TrimPrefix(uri, "/")}
	for _, p := range paramPattern.FindAllStringSubmatch(uri, -1) {
		r.Parameters = append(r.Parameters, manifest.Parameter{Name: p[1], Required: p[2] == ""})
	}
	for _, o := range opts {
		o(&r)
	}
	return r
}

func named(n string) func(*manifest.Route) { return func(r *manifest.Route) { r.Name = &n } }

func app(routes ...manifest.Route) *manifest.Manifest {
	return &manifest.Manifest{App: manifest.App{URL: ptr("http://localhost:8000")}, Routes: routes}
}

func urls(p Plan) []string {
	var out []string
	for _, t := range p.Targets {
		out = append(out, t.Method+" "+t.URL)
	}
	return out
}

func mustBuild(t *testing.T, m *manifest.Manifest, o Options) Plan {
	t.Helper()
	p, err := Build(m, o)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefaultsToGetRoutesOnly(t *testing.T) {
	p := mustBuild(t, app(route("GET /api/users"), route("POST /api/users"), route("DELETE /api/users/{user}"), route("GET /")), Options{})

	if got := strings.Join(urls(p), ", "); got != "GET http://localhost:8000/api/users, GET http://localhost:8000/" {
		t.Errorf("targets = %s", got)
	}
}

func TestSelectsByIdOrNameWithWildcards(t *testing.T) {
	m := app(route("GET /api/users", named("users.index")), route("POST /api/users", named("users.store")), route("GET /health"))

	p := mustBuild(t, m, Options{Routes: []string{"users.*", "GET /api/*"}}) // overlap is deduplicated

	if got := strings.Join(urls(p), ", "); got != "GET http://localhost:8000/api/users, POST http://localhost:8000/api/users" {
		t.Errorf("targets = %s", got)
	}
	if p.Targets[1].Name != "POST /api/users" {
		t.Errorf("target name = %q, want the route id", p.Targets[1].Name)
	}
}

func TestUnmatchedRoutePatternIsAnError(t *testing.T) {
	_, err := Build(app(route("GET /a")), Options{Routes: []string{"GET /nope"}})
	if err == nil || !strings.Contains(err.Error(), `"GET /nope" matches no route`) {
		t.Errorf("err = %v", err)
	}
}

func TestFillsAndEscapesParameters(t *testing.T) {
	m := app(route("GET /users/{user}/posts/{post}"))

	p := mustBuild(t, m, Options{Params: map[string]string{"user": "1", "post": "a b/c"}})

	if got := p.Targets[0].URL; got != "http://localhost:8000/users/1/posts/a%20b%2Fc" {
		t.Errorf("url = %s", got)
	}
}

func TestDropsMissingOptionalParameters(t *testing.T) {
	p := mustBuild(t, app(route("GET /posts/{slug?}"), route("GET /archive/{year?}/list")), Options{})

	if got := strings.Join(urls(p), ", "); got != "GET http://localhost:8000/posts, GET http://localhost:8000/archive/list" {
		t.Errorf("targets = %s", got)
	}
}

func TestSkipsRoutesMissingRequiredParameters(t *testing.T) {
	p := mustBuild(t, app(route("GET /users/{user}/posts/{post}"), route("GET /")), Options{Params: map[string]string{"post": "2"}})

	if len(p.Targets) != 1 || len(p.Skipped) != 1 || p.Skipped[0].Reason != "missing --param user" {
		t.Errorf("plan = %+v", p)
	}
}

func TestErrorsWhenNothingCanBeSent(t *testing.T) {
	_, err := Build(app(route("GET /users/{user}")), Options{})
	if err == nil || !strings.Contains(err.Error(), "GET /users/{user}: missing --param user") {
		t.Errorf("err = %v, want the skip reasons listed", err)
	}
}

func TestChecksWherePatterns(t *testing.T) {
	r := route("GET /users/{user}", func(r *manifest.Route) { r.Parameters[0].Pattern = ptr("[0-9]+") })

	p := mustBuild(t, app(r, route("GET /")), Options{Params: map[string]string{"user": "abc"}})
	if len(p.Skipped) != 1 || !strings.Contains(p.Skipped[0].Reason, "doesn't match the route's pattern [0-9]+") {
		t.Errorf("skipped = %+v", p.Skipped)
	}

	// Laravel matches the whole value, so "12a" fails too.
	p = mustBuild(t, app(r, route("GET /")), Options{Params: map[string]string{"user": "12a"}})
	if len(p.Skipped) != 1 {
		t.Errorf("partial match accepted: %+v", p)
	}

	p = mustBuild(t, app(r), Options{Params: map[string]string{"user": "12"}})
	if len(p.Targets) != 1 {
		t.Errorf("valid value rejected: %+v", p)
	}
}

func TestDomainRoutesUseTheirHostAndTheBasePort(t *testing.T) {
	r := route("GET /dashboard", func(r *manifest.Route) {
		r.Domain = ptr("{account}.app.test")
		r.Parameters = []manifest.Parameter{{Name: "account", Required: true}}
	})

	p := mustBuild(t, app(r), Options{Params: map[string]string{"account": "acme"}})

	if got := p.Targets[0].URL; got != "http://acme.app.test:8000/dashboard" {
		t.Errorf("url = %s", got)
	}
}

func TestBaseURL(t *testing.T) {
	m := app(route("GET /"))

	if p := mustBuild(t, m, Options{}); p.BaseURL != "http://localhost:8000" {
		t.Errorf("default base URL = %q, want the manifest's app.url", p.BaseURL)
	}
	if p := mustBuild(t, m, Options{BaseURL: "https://staging.example.com/sub/"}); p.Targets[0].URL != "https://staging.example.com/sub" || p.BaseURL != "https://staging.example.com/sub/" {
		t.Errorf("override + base path: %s (base %s)", p.Targets[0].URL, p.BaseURL)
	}

	m.App.URL = nil
	if _, err := Build(m, Options{}); err == nil || !strings.Contains(err.Error(), "--base-url") {
		t.Errorf("no base URL: err = %v", err)
	}
	if _, err := Build(m, Options{BaseURL: "localhost:8000"}); err == nil {
		t.Error("base URL without scheme should be rejected")
	}
}

func TestSendsHeadersToEveryTarget(t *testing.T) {
	h := http.Header{"Authorization": {"Bearer t"}}

	p := mustBuild(t, app(route("GET /a"), route("GET /b")), Options{Header: h})
	h.Set("Authorization", "changed later")

	for _, tg := range p.Targets {
		if tg.Header.Get("Authorization") != "Bearer t" {
			t.Errorf("%s header = %v", tg.Name, tg.Header)
		}
	}
}

func TestWarnsAboutLikely401And422(t *testing.T) {
	auth := func(r *manifest.Route) {
		r.MiddlewareResolved = []string{`Illuminate\Auth\Middleware\Authenticate:sanctum`}
	}
	validated := func(r *manifest.Route) {
		r.Request = manifest.Request{FormRequest: ptr(`App\StoreUserRequest`), Rules: map[string][]string{"name": {"required"}}}
	}
	unknownRules := func(r *manifest.Route) { r.Request = manifest.Request{FormRequest: ptr(`App\UpdateUserRequest`)} }
	noRules := func(r *manifest.Route) {
		r.Request = manifest.Request{FormRequest: ptr(`App\PingRequest`), Rules: map[string][]string{}}
	}
	m := app(route("GET /me", auth), route("POST /users", validated), route("PUT /users/{user}", unknownRules), route("POST /ping", noRules))

	p := mustBuild(t, m, Options{Routes: []string{"*"}, Params: map[string]string{"user": "1"}})
	if len(p.Warnings) != 3 || !strings.Contains(p.Warnings[0], "expect 401") ||
		!strings.Contains(p.Warnings[1], "POST /users: validates") || !strings.Contains(p.Warnings[2], "PUT /users/{user}: validates") {
		t.Errorf("warnings = %q", p.Warnings)
	}

	p = mustBuild(t, m, Options{Routes: []string{"GET /me"}, Header: http.Header{"Authorization": {"Bearer t"}}})
	if len(p.Warnings) != 0 {
		t.Errorf("token given, warnings = %q", p.Warnings)
	}
}
