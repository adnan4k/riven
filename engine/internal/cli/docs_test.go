package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocsWritesOpenAPI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openapi.json")

	_, stderr, err := run(t, "docs", "-m", sampleManifest, "-o", path, "--title", "Users API", "--api-version", "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "Wrote "+path+" (6 routes)") {
		t.Errorf("stderr = %q", stderr)
	}

	var doc struct {
		OpenAPI string `json:"openapi"`
		Info    struct{ Title, Version string }
		Paths   map[string]any
	}
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.OpenAPI != "3.1.0" || doc.Info.Title != "Users API" || doc.Info.Version != "2.0.0" || doc.Paths["/api/users/{user}"] == nil {
		t.Errorf("doc = %+v", doc)
	}
}

func TestDocsToStdout(t *testing.T) {
	stdout, _, err := run(t, "docs", "-m", sampleManifest)
	if err != nil || !strings.HasPrefix(stdout, "{\n") || !json.Valid([]byte(stdout)) {
		t.Errorf("err = %v, stdout starts %q", err, stdout[:min(len(stdout), 40)])
	}
}
