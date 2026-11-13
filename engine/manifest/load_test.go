package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimal builds a valid v1 manifest around the given routes JSON.
func minimal(version, routes string) string {
	return `{"schema_version": ` + version + `,
		"generator": {"name": "faysal/riven", "version": null},
		"generated_at": "2026-11-13T12:00:00Z",
		"app": {"name": null, "url": null, "laravel_version": "13.42.0", "php_version": "8.4.24"},
		"routes": [` + routes + `], "warnings": []}`
}

func route(rules string) string {
	return `{"id": "POST /users", "uri": "users", "methods": ["POST"], "name": null, "domain": null,
		"action": {"type": "closure", "class": null, "method": null},
		"middleware": [], "middleware_resolved": [], "parameters": [],
		"request": {"form_request": "App\\Http\\Requests\\StoreUserRequest", "rules": ` + rules + `},
		"response": {"resource": null, "collection": null}}`
}

func TestLoadsRealExport(t *testing.T) {
	m, err := Load("testdata/riven-app.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(m.Routes) != 6 || len(m.Warnings) != 1 {
		t.Fatalf("got %d routes, %d warnings; want 6, 1", len(m.Routes), len(m.Warnings))
	}
	if m.App.LaravelVersion == "" || m.GeneratedAt.IsZero() {
		t.Errorf("app/generated_at not decoded: %+v, %v", m.App, m.GeneratedAt)
	}

	store := findRoute(t, m, "POST /api/users")
	if store.Action.Type != ActionController || *store.Action.Method != "store" {
		t.Errorf("action = %+v", store.Action)
	}
	if got := store.Request.Rules["name"]; strings.Join(got, "|") != "required|string|max:255" {
		t.Errorf("rules[name] = %v", got)
	}
	if store.Response.Resource == nil || *store.Response.Collection {
		t.Errorf("response = %+v, want single UserResource", store.Response)
	}

	update := findRoute(t, m, "PUT /api/users/{user}")
	if update.Request.Rules != nil {
		t.Errorf("rules needing a live request should be nil, got %v", update.Request.Rules)
	}
	if update.Parameters[0].Name != "user" || !update.Parameters[0].Required || update.Parameters[0].Pattern != nil {
		t.Errorf("parameters = %+v", update.Parameters)
	}
	if m.Warnings[0].Code != WarnRulesNeedRequest || m.Warnings[0].RouteID != update.ID {
		t.Errorf("warning = %+v", m.Warnings[0])
	}

	closure := findRoute(t, m, "GET /")
	if closure.Action.Type != ActionClosure || closure.Action.Class != nil || closure.Name != nil {
		t.Errorf("closure route = %+v", closure)
	}
}

func TestNullRulesAreNilAndEmptyRulesAreEmpty(t *testing.T) {
	for _, tc := range []struct {
		rules   string
		wantNil bool
	}{
		{"null", true},
		{"{}", false},
	} {
		m, err := Parse(strings.NewReader(minimal("1", route(tc.rules))))
		if err != nil {
			t.Fatalf("rules %s: %v", tc.rules, err)
		}
		rules := m.Routes[0].Request.Rules
		if (rules == nil) != tc.wantNil || len(rules) != 0 {
			t.Errorf("rules %s: got %#v, want nil=%v", tc.rules, rules, tc.wantNil)
		}
	}
}

func TestRejectsUnsupportedVersions(t *testing.T) {
	for _, tc := range []struct {
		version, hint string
	}{
		{"2", "upgrade riven-engine"},
		{"0", "riven:export"},
	} {
		_, err := Parse(strings.NewReader(minimal(tc.version, "")))

		var verr *VersionError
		if !errors.As(err, &verr) {
			t.Fatalf("v%s: err = %v, want *VersionError", tc.version, err)
		}
		if !strings.Contains(err.Error(), tc.hint) {
			t.Errorf("v%s: %q should tell the user to %q", tc.version, err, tc.hint)
		}
	}
}

func TestVersionIsCheckedBeforeTheRestIsDecoded(t *testing.T) {
	// A v2 file whose shape no longer fits v1 structs must still report the version.
	_, err := Parse(strings.NewReader(`{"schema_version": 2, "routes": {"changed": "shape"}}`))

	var verr *VersionError
	if !errors.As(err, &verr) || verr.Got != 2 {
		t.Fatalf("err = %v, want VersionError{Got: 2}", err)
	}
}

func TestIgnoresUnknownFields(t *testing.T) {
	// Additive changes keep schema_version, so newer collectors must still load.
	input := strings.Replace(minimal("1", ""), `"warnings": []`, `"warnings": [], "resources": []`, 1)

	if _, err := Parse(strings.NewReader(input)); err != nil {
		t.Fatalf("unknown field should be ignored: %v", err)
	}
}

func TestRejectsNonManifests(t *testing.T) {
	if _, err := Parse(strings.NewReader(`{"name": "laravel/laravel"}`)); !errors.Is(err, ErrNotManifest) {
		t.Errorf("missing schema_version: err = %v, want ErrNotManifest", err)
	}
	if _, err := Parse(strings.NewReader(`{"schema_version": 1,`)); err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("broken JSON: err = %v", err)
	}
}

func TestLoadMissingFileHintsAtExport(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "riven.json"))

	if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "riven:export") {
		t.Errorf("err = %v, want ErrNotExist with an export hint", err)
	}
}

func TestLoadPrefixesErrorsWithPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "riven.json")
	if err := os.WriteFile(path, []byte(minimal("2", "")), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)

	var verr *VersionError
	if !errors.As(err, &verr) || !strings.HasPrefix(err.Error(), path+": ") {
		t.Errorf("err = %v, want path-prefixed VersionError", err)
	}
}

func findRoute(t *testing.T, m *Manifest, id string) Route {
	t.Helper()
	for _, r := range m.Routes {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("route %q not in manifest", id)
	return Route{}
}
