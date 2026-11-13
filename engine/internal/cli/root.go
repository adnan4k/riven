// Package cli wires commands to flags and output. Logic lives in other
// packages so it can be tested without going through Cobra.
package cli

import "github.com/spf13/cobra"

// Version is set at build time: go build -ldflags "-X github.com/faysal/riven/engine/internal/cli.Version=v0.1.0".
var Version = "dev"

// NewRootCommand builds a fresh command tree, so tests don't share flag state.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:          "riven-engine",
		Short:        "Work with a Laravel app's riven.json manifest",
		Version:      Version,
		SilenceUsage: true, // don't print usage on runtime errors, only on bad flags
	}

	root.PersistentFlags().StringP("manifest", "m", "riven.json", "path to the manifest from `php artisan riven:export`")

	return root
}
