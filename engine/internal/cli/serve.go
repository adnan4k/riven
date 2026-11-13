package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/adnan4k/riven/engine/server"
	"github.com/adnan4k/riven/engine/web"
	"github.com/spf13/cobra"
)

// DefaultReportsDir is where `stress --save` writes and `serve` reads.
const DefaultReportsDir = "riven-reports"

func newServeCommand() *cobra.Command {
	var addr, reports, baseURL, appDir string
	var watchFiles bool

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Open the dashboard: routes, docs and stress reports",
		Long: `Serve the dashboard on this machine only (127.0.0.1) by default: it shows
your app's routes and internals. Files are re-read on every request, so
re-running riven:export or a stress test shows up after a reload.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, _ := cmd.Flags().GetString("manifest")

			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("listen on %s: %w", addr, err)
			}

			if appDir == "" {
				appDir, _ = os.Getwd()
			}
			host, _, _ := net.SplitHostPort(ln.Addr().String())
			ip := net.ParseIP(host)
			dash := server.New(server.Config{
				ManifestPath: path, ReportsDir: reports, UI: web.Dist(),
				BaseURL: baseURL, AppDir: appDir, Version: Version,
				// Listening on a LAN address means other hosts are expected.
				AllowAnyHost: ip == nil || !ip.IsLoopback(),
				AutoExport:   watchFiles,
			})
			srv := &http.Server{
				Handler:           dash,
				ReadHeaderTimeout: 10 * time.Second,
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()
			dash.Watch(ctx)
			srv.RegisterOnShutdown(dash.Close) // end open event streams so shutdown doesn't wait on them
			go func() {
				<-ctx.Done()
				shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				srv.Shutdown(shutdown)
			}()

			fmt.Fprintf(cmd.OutOrStdout(), "Riven dashboard on http://%s (Ctrl-C to stop)\n", ln.Addr())
			if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7600", "address to listen on (use 0.0.0.0:7600 to share on your network)")
	cmd.Flags().StringVar(&reports, "reports", DefaultReportsDir, "directory with stress reports (from stress --save)")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "app URL for Try it and stress runs (default: app.url from the manifest)")
	cmd.Flags().StringVar(&appDir, "app-dir", "", "Laravel app root for Rescan routes (default: current directory)")
	cmd.Flags().BoolVar(&watchFiles, "watch", true, "re-run riven:export when the app's PHP files change, and update open dashboards")
	return cmd
}
