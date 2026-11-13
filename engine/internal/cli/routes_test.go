package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := NewRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

func writeManifest(t *testing.T, routes, warnings string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "riven.json")
	body := `{"schema_version": 1, "generator": {"name": "faysal/riven", "version": null},
		"generated_at": "2026-11-13T12:00:00Z",
		"app": {"name": null, "url": null, "laravel_version": "13.42.0", "php_version": "8.4.24"},
		"routes": [` + routes + `], "warnings": [` + warnings + `]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const controllerRoute = `{"id": "PUT /users/{user}", "uri": "users/{user}", "methods": ["PUT", "PATCH"], "name": "users.update", "domain": null,
	"action": {"type": "controller", "class": "App\\UserController", "method": "update"},
	"middleware": ["api", "auth"], "middleware_resolved": [], "parameters": [],
	"request": {"form_request": null, "rules": null}, "response": {"resource": null, "collection": null}}`

const closureRoute = `{"id": "GET /", "uri": "/", "methods": ["GET", "HEAD"], "name": null, "domain": null,
	"action": {"type": "closure", "class": null, "method": null},
	"middleware": ["web"], "middleware_resolved": [], "parameters": [],
	"request": {"form_request": null, "rules": null}, "response": {"resource": null, "collection": null}}`

func TestRoutesPrintsTable(t *testing.T) {
	path := writeManifest(t, controllerRoute+","+closureRoute,
		`{"route_id": "PUT /users/{user}", "code": "rules_need_request", "message": "needs a request"}`)

	stdout, stderr, err := run(t, "routes", "--manifest", path)
	if err != nil {
		t.Fatal(err)
	}

	// Exact layout is covered by internal/table and the golden-file test.
	for _, cell := range []string{"| users/{user} |", "PUT|PATCH", "users.update", `App\UserController@update`, "api, auth", "| Closure ", "web"} {
		if !strings.Contains(stdout, cell) {
			t.Errorf("stdout missing %q:\n%s", cell, stdout)
		}
	}
	if stderr != "PUT /users/{user}: needs a request\n" {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestRoutesWithNoRoutes(t *testing.T) {
	stdout, _, err := run(t, "routes", "-m", writeManifest(t, "", ""))
	if err != nil || stdout != "No routes found.\n" {
		t.Errorf("stdout = %q, err = %v", stdout, err)
	}
}

func TestRoutesReportsLoadErrors(t *testing.T) {
	_, _, err := run(t, "routes", "-m", filepath.Join(t.TempDir(), "missing.json"))
	if err == nil || !strings.Contains(err.Error(), "riven:export") {
		t.Errorf("err = %v, want a hint to run riven:export", err)
	}
}
