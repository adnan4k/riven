package rules

import (
	"encoding/json"
	"slices"
	"testing"
)

// prop returns the JSON form of one property, without x-laravel-rules, for compact comparisons.
func prop(t *testing.T, rules map[string][]string, path ...string) string {
	t.Helper()
	s := ToSchema(rules).Schema
	for _, p := range path {
		if p == "[]" {
			s = s["items"].(Schema)
			continue
		}
		s = s["properties"].(Schema)[p].(Schema)
	}
	clean := Schema{}
	for k, v := range s {
		if k != "x-laravel-rules" {
			clean[k] = v
		}
	}
	b, _ := json.Marshal(clean)
	return string(b)
}

func TestFieldRules(t *testing.T) {
	for _, tc := range []struct {
		rules []string
		want  string
	}{
		{[]string{"required", "string", "max:255"}, `{"maxLength":255,"type":"string"}`},
		{[]string{"min:3"}, `{"minLength":3}`}, // no type: Laravel compares string length
		{[]string{"integer", "between:1,10"}, `{"maximum":10,"minimum":1,"type":"integer"}`},
		{[]string{"numeric", "min:0.5"}, `{"minimum":0.5,"type":"number"}`},
		{[]string{"array", "size:3"}, `{"maxItems":3,"minItems":3,"type":"array"}`},
		{[]string{"nullable", "boolean"}, `{"type":["boolean","null"]}`},
		{[]string{"nullable"}, `{}`},
		{[]string{"email"}, `{"format":"email","type":"string"}`},
		{[]string{"url"}, `{"format":"uri","type":"string"}`},
		{[]string{"uuid"}, `{"format":"uuid","type":"string"}`},
		{[]string{"date"}, `{"type":"string"}`},
		{[]string{"date_format:Y-m-d"}, `{"format":"date","type":"string"}`},
		{[]string{"in:draft,published"}, `{"enum":["draft","published"]}`},
		{[]string{`in:"a","b, c"`}, `{"enum":["a","b, c"]}`}, // Rule::in() quoting
		{[]string{"integer", "in:1,2"}, `{"enum":[1,2],"type":"integer"}`},
		{[]string{"string", "not_in:admin"}, `{"not":{"enum":["admin"]},"type":"string"}`},
		{[]string{"regex:/^[a-z]+$/"}, `{"pattern":"^[a-z]+$"}`},
		{[]string{"alpha_num"}, `{"pattern":"^[\\p{L}\\p{M}\\p{N}]+$"}`},
		{[]string{"image", "max:2048"}, `{"format":"binary","type":"string"}`}, // kilobytes: not a length
	} {
		got := prop(t, map[string][]string{"f": tc.rules}, "f")
		if got != tc.want {
			t.Errorf("%v\n got %s\nwant %s", tc.rules, got, tc.want)
		}
	}
}

func TestRequiredAndRawRules(t *testing.T) {
	r := ToSchema(map[string][]string{
		"name":  {"required", "string"},
		"bio":   {"sometimes", "string"},
		"email": {"required_if:type,user", "email"},
	})

	if req := r.Schema["required"]; !slices.Equal(req.([]string), []string{"name"}) {
		t.Errorf("required = %v, want [name] (conditional rules aren't required)", req)
	}
	if r.Schema["type"] != "object" {
		t.Errorf("root type = %v", r.Schema["type"])
	}
	raw := r.Schema["properties"].(Schema)["name"].(Schema)["x-laravel-rules"]
	if !slices.Equal(raw.([]string), []string{"required", "string"}) {
		t.Errorf("x-laravel-rules = %v", raw)
	}
}

func TestUnmappedRulesAreReported(t *testing.T) {
	r := ToSchema(map[string][]string{
		"email":    {"required", "email", "unique:users,email"},
		"password": {"required", `Illuminate\Validation\Rules\Password`},
		"terms":    {"accepted"},
		"code":     {"regex:/^[a-z]+$/i"}, // flags: not translated
	})

	want := []string{
		"code: regex:/^[a-z]+$/i",
		"email: unique:users,email",
		`password: Illuminate\Validation\Rules\Password`,
		"terms: accepted",
	}
	if !slices.Equal(r.Unmapped, want) {
		t.Errorf("unmapped = %q\nwant %q", r.Unmapped, want)
	}
}

func TestNestedObjectsAndArrays(t *testing.T) {
	rules := map[string][]string{
		"address":         {"required", "array"},
		"address.city":    {"required", "string"},
		"address.zip":     {"string"},
		"items":           {"required", "array", "min:1"},
		"items.*.id":      {"required", "integer"},
		"items.*.qty":     {"integer", "min:1"},
		"tags.*":          {"string", "max:20"},
		"meta.source.app": {"string"},
	}

	if got := prop(t, rules, "address"); got != `{"properties":{"city":{"type":"string","x-laravel-rules":["required","string"]},"zip":{"type":"string","x-laravel-rules":["string"]}},"required":["city"],"type":"object"}` {
		t.Errorf("address = %s", got)
	}
	if got := prop(t, rules, "items", "[]"); got != `{"properties":{"id":{"type":"integer","x-laravel-rules":["required","integer"]},"qty":{"minimum":1,"type":"integer","x-laravel-rules":["integer","min:1"]}},"required":["id"],"type":"object"}` {
		t.Errorf("items[] = %s", got)
	}
	if got := prop(t, rules, "items"); !json.Valid([]byte(got)) || ToSchema(rules).Schema["properties"].(Schema)["items"].(Schema)["minItems"] != int64(1) {
		t.Errorf("items = %s", got)
	}
	if got := prop(t, rules, "tags", "[]"); got != `{"maxLength":20,"type":"string"}` {
		t.Errorf("tags[] = %s (tags has no rule of its own, so it's inferred as an array)", got)
	}
	if got := prop(t, rules, "meta", "source", "app"); got != `{"type":"string"}` {
		t.Errorf("meta.source.app = %s", got)
	}
}

func TestConfirmedAddsConfirmationField(t *testing.T) {
	r := ToSchema(map[string][]string{"password": {"required", "string", "min:8", "confirmed"}})

	props := r.Schema["properties"].(Schema)
	if props["password_confirmation"] == nil || !slices.Equal(r.Schema["required"].([]string), []string{"password", "password_confirmation"}) {
		t.Errorf("schema = %v", r.Schema)
	}
}

func TestFilesNeedMultipart(t *testing.T) {
	if !ToSchema(map[string][]string{"avatar": {"file"}}).Multipart {
		t.Error("file rule should require multipart")
	}
	if ToSchema(map[string][]string{"name": {"string"}}).Multipart {
		t.Error("plain fields should stay JSON")
	}
}

func TestRivenAppRules(t *testing.T) {
	// StoreUserRequest from riven-app, as exported in the sample manifest.
	r := ToSchema(map[string][]string{
		"name":     {"required", "string", "max:255"},
		"email":    {"required", "email", "unique:users,email"},
		"password": {"required", `Illuminate\Validation\Rules\Password`},
	})

	b, _ := json.Marshal(r.Schema)
	want := `{"properties":{"email":{"format":"email","type":"string","x-laravel-rules":["required","email","unique:users,email"]},"name":{"maxLength":255,"type":"string","x-laravel-rules":["required","string","max:255"]},"password":{"x-laravel-rules":["required","Illuminate\\Validation\\Rules\\Password"]}},"required":["email","name","password"],"type":"object"}`
	if string(b) != want {
		t.Errorf("got  %s\nwant %s", b, want)
	}
}

func TestEmptyRules(t *testing.T) {
	b, _ := json.Marshal(ToSchema(map[string][]string{}).Schema)
	if string(b) != `{}` {
		t.Errorf("empty rules = %s, want {} (no fields known, nothing constrained)", b)
	}
}
