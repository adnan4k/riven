package openapi

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/faysal/riven/engine/manifest"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata/")

const oasSchemaID = "https://spec.openapis.org/oas/3.1/schema/2022-10-07"

func ptr[T any](v T) *T { return &v }

// encode round-trips the doc through JSON, as a consumer would see it.
func encode(t *testing.T, doc Doc) (data []byte, generic map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatal(err)
	}
	return data, generic
}

// validate checks the document against the official OpenAPI 3.1 schema, then
// compiles every embedded Schema Object as JSON Schema 2020-12 (the OAS
// schema doesn't look inside them), then checks rules no JSON Schema can:
// path templates vs path parameters, unique operationIds.
func validate(t *testing.T, doc Doc) {
	t.Helper()
	data, generic := encode(t, doc)

	c := jsonschema.NewCompiler()
	oas, err := os.Open("testdata/oas-3.1-schema.json")
	if err != nil {
		t.Fatal(err)
	}
	defer oas.Close()
	oasDoc, err := jsonschema.UnmarshalJSON(oas)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddResource(oasSchemaID, oasDoc); err != nil {
		t.Fatal(err)
	}
	instance, _ := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err := c.MustCompile(oasSchemaID).Validate(instance); err != nil {
		t.Fatalf("not a valid OpenAPI 3.1 document:\n%v", err)
	}

	const docURL = "mem://openapi.json"
	if err := c.AddResource(docURL, instance); err != nil {
		t.Fatal(err)
	}
	for _, ptr := range schemaPointers(generic, "") {
		if _, err := c.Compile(docURL + "#" + ptr); err != nil {
			t.Errorf("schema at %s doesn't compile: %v", ptr, err)
		}
	}

	ids := map[string]string{}
	paths, _ := generic["paths"].(map[string]any)
	for path, item := range paths {
		want := paramNames(path)
		for method, op := range item.(map[string]any) {
			op, ok := op.(map[string]any)
			if !ok { // "servers"
				continue
			}
			var got []string
			params, _ := op["parameters"].([]any)
			for _, p := range params {
				if p := p.(map[string]any); p["in"] == "path" {
					got = append(got, p["name"].(string))
				}
			}
			if !slices.Equal(got, want) {
				t.Errorf("%s %s: path parameters %v, template has %v", method, path, got, want)
			}
			id := op["operationId"].(string)
			if prev, dup := ids[id]; dup {
				t.Errorf("operationId %q used by %s and %s %s", id, prev, method, path)
			}
			ids[id] = method + " " + path
		}
	}
}

func paramNames(path string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`\{(\w+)\}`).FindAllStringSubmatch(path, -1) {
		out = append(out, m[1])
	}
	return out
}

// schemaPointers finds JSON pointers to every "schema" value and component schema.
func schemaPointers(v any, at string) []string {
	var out []string
	switch v := v.(type) {
	case map[string]any:
		for k, child := range v {
			p := at + "/" + strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
			if k == "schema" || strings.HasPrefix(at, "/components/schemas") && !strings.Contains(at[len("/components/schemas"):], "/") && at != "/components/schemas" {
				out = append(out, p)
			}
			out = append(out, schemaPointers(child, p)...)
		}
	case []any:
		for i, child := range v {
			out = append(out, schemaPointers(child, fmt.Sprintf("%s/%d", at, i))...)
		}
	}
	return out
}

func get(t *testing.T, v any, path ...string) any {
	t.Helper()
	for _, p := range path {
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("%v: not an object at %q", path, p)
		}
		v, ok = m[p]
		if !ok {
			t.Fatalf("%v: no key %q", path, p)
		}
	}
	return v
}

func TestSampleManifestIsValidOpenAPI(t *testing.T) {
	m, err := manifest.Load("../testdata/riven-app.json")
	if err != nil {
		t.Fatal(err)
	}
	doc := Build(m, Options{})
	validate(t, doc)

	data, _ := encode(t, doc)
	golden := filepath.Join("testdata", "riven-app.openapi.json")
	if *update {
		os.WriteFile(golden, append(data, '\n'), 0o644)
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run `go test ./openapi -update`)", err)
	}
	if string(append(data, '\n')) != string(want) {
		t.Errorf("output differs from %s (run with -update if intended)", golden)
	}
}

// edgeCases is a manifest with every shape the builder handles.
func edgeCases() *manifest.Manifest {
	ctrl := func(class, method string) manifest.Action {
		return manifest.Action{Type: manifest.ActionController, Class: &class, Method: &method}
	}
	closure := manifest.Action{Type: manifest.ActionClosure}
	auth := []string{`Illuminate\Auth\Middleware\Authenticate:sanctum`}
	fr := func(rules map[string][]string) manifest.Request {
		return manifest.Request{FormRequest: ptr(`App\Http\Requests\SomeRequest`), Rules: rules}
	}
	resource := func(class string, coll bool) manifest.Response {
		return manifest.Response{Resource: &class, Collection: &coll}
	}

	return &manifest.Manifest{
		App: manifest.App{Name: ptr("edge"), URL: ptr("https://edge.test:8443"), LaravelVersion: "13.0.0"},
		Routes: []manifest.Route{
			{ID: "GET /posts/{slug?}", URI: "posts/{slug?}", Methods: []string{"GET", "HEAD"}, Name: ptr("posts.show"), Action: closure,
				Parameters: []manifest.Parameter{{Name: "slug", Required: false}}},
			{ID: "PUT /users/{user}", URI: "users/{user}", Methods: []string{"PUT", "PATCH"}, Name: ptr("users.update"),
				Action: ctrl(`App\Http\Controllers\UserController`, "update"), MiddlewareResolved: auth,
				Parameters: []manifest.Parameter{{Name: "user", Required: true, Pattern: ptr("[0-9]+")}},
				Request:    manifest.Request{FormRequest: ptr(`App\Http\Requests\UpdateUserRequest`)}, // rules unknown
				Response:   resource(`App\Http\Resources\UserResource`, false)},
			{ID: "POST /users", URI: "users", Methods: []string{"POST"}, Action: ctrl(`App\Http\Controllers\UserController`, "store"),
				Request:  fr(map[string][]string{"name": {"required", "string"}, "avatar": {"nullable", "image"}, "password": {"required", "confirmed"}}),
				Response: resource(`App\Http\Resources\UserResource`, false)},
			{ID: "GET /search", URI: "search", Methods: []string{"GET", "HEAD"}, Action: closure,
				Request: fr(map[string][]string{"q": {"required", "string", "min:2"}, "page": {"integer"}})},
			{ID: "GET /admin/users", URI: "admin/users", Methods: []string{"GET", "HEAD"}, Action: closure,
				Response: resource(`App\Admin\Resources\UserResource`, true)}, // same short name, other namespace
			{ID: "GET /dashboard", URI: "dashboard", Methods: []string{"GET", "HEAD"}, Domain: ptr("{account}.edge.test"), Action: closure,
				Parameters: []manifest.Parameter{{Name: "account", Required: true}}},
			{ID: "POST /ping", URI: "ping", Methods: []string{"POST"}, Action: closure, Request: fr(map[string][]string{})},
		},
	}
}

func TestEdgeCasesAreValidOpenAPI(t *testing.T) {
	validate(t, Build(edgeCases(), Options{Title: "Edge", Version: "2.0.0"}))
}

func TestEdgeCases(t *testing.T) {
	_, doc := encode(t, Build(edgeCases(), Options{}))
	paths := get(t, doc, "paths").(map[string]any)

	// Optional parameter: both paths, each with its own operationId.
	if get(t, paths, "/posts", "get", "operationId") != "posts.show" || get(t, paths, "/posts/{slug}", "get", "operationId") != "posts.show.get" {
		t.Errorf("optional param paths = %v", slices.Sorted(maps.Keys(paths)))
	}

	// PUT|PATCH: two operations; HEAD dropped; where() pattern anchored; auth.
	user := get(t, paths, "/users/{user}").(map[string]any)
	if user["put"] == nil || user["patch"] == nil || user["head"] != nil {
		t.Errorf("users/{user} operations = %v", slices.Sorted(maps.Keys(user)))
	}
	if get(t, paths, "/posts").(map[string]any)["head"] != nil {
		t.Error("HEAD should be left out (implied by GET)")
	}
	if p := get(t, user, "put", "parameters").([]any)[0]; get(t, p, "schema", "pattern") != "^(?:[0-9]+)$" {
		t.Errorf("param = %v", p)
	}
	if get(t, user, "put", "responses", "401", "description") != "Unauthenticated" || get(t, doc, "components", "securitySchemes", "bearerAuth", "scheme") != "bearer" {
		t.Error("auth route should have a 401 and bearer security")
	}
	if get(t, user, "put", "requestBody", "x-riven-rules-unknown") != true || get(t, user, "put", "responses", "200", "content") == nil {
		t.Error("unknown rules: body marked; resource: 200 with content")
	}

	// File upload => multipart; POST + resource => 201; confirmed => extra field; 422.
	store := get(t, paths, "/users", "post")
	props := get(t, store, "requestBody", "content", "multipart/form-data", "schema", "properties").(map[string]any)
	if props["password_confirmation"] == nil || get(t, props, "avatar", "format") != "binary" {
		t.Errorf("store body = %v", props)
	}
	if get(t, store, "responses", "201", "content", "application/json", "schema", "properties", "data", "$ref") != "#/components/schemas/UserResource" {
		t.Error("POST with a resource should answer 201 with {data: UserResource}")
	}
	if get(t, store, "responses", "422", "content", "application/json", "schema", "required") == nil {
		t.Error("FormRequest route should document the 422 shape")
	}

	// GET with rules => query parameters, no body.
	search := get(t, paths, "/search", "get").(map[string]any)
	if search["requestBody"] != nil {
		t.Error("GET should not have a request body")
	}
	q := get(t, search, "parameters").([]any)
	if len(q) != 2 || get(t, q[1], "name") != "q" || get(t, q[1], "required") != true || get(t, q[0], "in") != "query" {
		t.Errorf("query params = %v", q)
	}

	// Same short name in two namespaces => second gets the full name.
	if get(t, paths, "/admin/users", "get", "responses", "200", "content", "application/json", "schema", "properties", "data", "items", "$ref") != `#/components/schemas/App.Admin.Resources.UserResource` {
		t.Error("colliding resource names should be disambiguated")
	}

	// Domain route => server with a variable, base port kept.
	srv := get(t, paths, "/dashboard", "servers").([]any)[0]
	if get(t, srv, "url") != "https://{account}.edge.test:8443" || get(t, srv, "variables", "account", "default") != "account" {
		t.Errorf("domain server = %v", srv)
	}

	// FormRequest with no rules => optional, empty object body.
	if get(t, paths, "/ping", "post", "requestBody", "required") != false {
		t.Error("no rules => body not required")
	}
}
