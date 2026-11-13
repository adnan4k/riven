package cli

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// go test ./internal/cli -update rewrites the golden files from current output.
// Review the diff before committing: the golden file *is* the expected output.
var update = flag.Bool("update", false, "rewrite golden files in testdata/")

const sampleManifest = "../../testdata/riven-app.json"

// sampleAbs is the sample manifest's absolute path, for tests that chdir.
func sampleAbs(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(sampleManifest)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRoutesGolden(t *testing.T) {
	stdout, stderr, err := run(t, "routes", "--manifest", sampleManifest)
	if err != nil {
		t.Fatal(err)
	}

	assertGolden(t, "routes.stdout.golden", stdout)
	assertGolden(t, "routes.stderr.golden", stderr)
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)

	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run `go test ./internal/cli -update` to create it)", err)
	}
	if got != string(want) {
		t.Errorf("%s mismatch (run with -update if the change is intended)\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
