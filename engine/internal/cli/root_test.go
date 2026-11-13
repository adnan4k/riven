package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestManifestFlagDefaultsToRivenJSON(t *testing.T) {
	flag := NewRootCommand().PersistentFlags().Lookup("manifest")

	if flag == nil {
		t.Fatal("root command has no --manifest flag")
	}
	if flag.DefValue != "riven.json" || flag.Shorthand != "m" {
		t.Errorf("--manifest = (default %q, short %q), want (\"riven.json\", \"m\")", flag.DefValue, flag.Shorthand)
	}
}

func TestVersionFlag(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "dev") {
		t.Errorf("version output = %q, want it to contain %q", out.String(), "dev")
	}
}
