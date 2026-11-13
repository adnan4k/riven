package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/adnan4k/riven/engine/manifest"
	"github.com/adnan4k/riven/engine/openapi"
	"github.com/spf13/cobra"
)

func newDocsCommand() *cobra.Command {
	var output, title, version string

	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate an OpenAPI 3.1 document from the manifest",
		Long: `Generate an OpenAPI 3.1 document using only what the manifest knows:
routes, parameters, FormRequest rules, Resources and auth middleware.`,
		Example: `  riven-engine docs -o openapi.json
  riven-engine docs --title "Shop API" --api-version 2.1.0 > openapi.json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, _ := cmd.Flags().GetString("manifest")
			m, err := manifest.Load(path)
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(openapi.Build(m, openapi.Options{Title: title, Version: version}), "", "  ")
			if err != nil {
				return fmt.Errorf("encode OpenAPI document: %w", err)
			}
			data = append(data, '\n')

			if output == "" || output == "-" {
				_, err = cmd.OutOrStdout().Write(data)
				return err
			}
			if err := os.WriteFile(output, data, 0o644); err != nil {
				return fmt.Errorf("write OpenAPI document: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %s (%d routes)\n", output, len(m.Routes))
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "file to write (default: stdout)")
	cmd.Flags().StringVar(&title, "title", "", "info.title (default: the app name)")
	cmd.Flags().StringVar(&version, "api-version", "", `info.version, your API's version (default "1.0.0")`)
	return cmd
}
