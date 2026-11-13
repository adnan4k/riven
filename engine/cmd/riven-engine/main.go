// Command riven-engine reads the riven.json manifest exported by the PHP
// collector (php artisan riven:export) and works on it outside the app.
package main

import (
	"os"

	"github.com/faysal/riven/engine/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
