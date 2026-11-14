package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/adnan4k/riven/engine/web"
	"github.com/spf13/cobra"
)

func newUICommand() *cobra.Command {
	return &cobra.Command{
		Use:    "ui <dir>",
		Short:  "Write the built dashboard into a directory",
		Long:   "Write the dashboard compiled into this binary into <dir>, so the Laravel app can serve it at /riven.",
		Args:   cobra.ExactArgs(1),
		Hidden: true, // used by the PHP package, not by people
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := extractUI(web.Dist(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d files to %s\n", n, args[0])
			return nil
		},
	}
}

// extractUI copies the dashboard files into dir. It fails when the binary was
// built without the UI, so the caller can say how to build it.
func extractUI(ui fs.FS, dir string) (int, error) {
	if _, err := fs.Stat(ui, "index.html"); err != nil {
		return 0, errors.New("this riven-engine was built without the dashboard (cd engine/web && npm run build, then rebuild)")
	}
	n := 0
	err := fs.WalkDir(ui, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if d.Name() == ".gitkeep" {
			return nil
		}
		data, err := fs.ReadFile(ui, path)
		if err != nil {
			return err
		}
		n++
		return os.WriteFile(target, data, 0o644)
	})
	return n, err
}
