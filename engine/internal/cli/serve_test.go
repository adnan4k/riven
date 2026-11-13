package cli

import (
	"testing"
)

func TestServeDefaultsToLocalhost(t *testing.T) {
	flag := newServeCommand().Flags().Lookup("addr")
	if flag.DefValue != "127.0.0.1:7600" {
		t.Errorf("--addr default = %q, want 127.0.0.1:7600 (local only)", flag.DefValue)
	}
}
