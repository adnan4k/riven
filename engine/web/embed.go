// Package web holds the dashboard's built files (npm run build → dist/),
// compiled into the binary. dist/ is not committed except for .gitkeep, so
// a fresh checkout still builds; the server then shows how to build the UI.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the built dashboard, rooted at dist/.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist/ is always embedded, so this can't happen
	}
	return sub
}
