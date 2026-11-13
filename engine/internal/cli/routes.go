package cli

import (
	"fmt"
	"strings"

	"github.com/faysal/riven/engine/internal/table"
	"github.com/faysal/riven/engine/manifest"
	"github.com/spf13/cobra"
)

func newRoutesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "routes",
		Short: "List the routes in the manifest (same table as php artisan riven:scan)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, _ := cmd.Flags().GetString("manifest")

			m, err := manifest.Load(path)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(m.Routes) == 0 {
				_, err := fmt.Fprintln(out, "No routes found.")
				return err
			}

			if err := table.Render(out, []string{"URI", "Methods", "Name", "Action", "Middleware"}, routeRows(m.Routes)); err != nil {
				return err
			}

			// Warnings go to stderr so `riven-engine routes > routes.txt` keeps only the table.
			for _, w := range m.Warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s\n", w.RouteID, w.Message)
			}

			return nil
		},
	}
}

func routeRows(routes []manifest.Route) [][]string {
	rows := make([][]string, 0, len(routes))
	for _, r := range routes {
		rows = append(rows, []string{
			r.URI,
			strings.Join(r.Methods, "|"),
			deref(r.Name),
			actionLabel(r.Action),
			strings.Join(r.Middleware, ", "),
		})
	}
	return rows
}

func actionLabel(a manifest.Action) string {
	if a.Type == manifest.ActionClosure {
		return "Closure"
	}
	return deref(a.Class) + "@" + deref(a.Method)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
