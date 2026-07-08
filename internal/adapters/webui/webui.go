// Package webui holds the design system shared by the dashboard and admin
// panels — one stylesheet under /assets/, so the two feel like one
// product instead of two independently-skinned surfaces. Panel-specific
// styles (charts, stat tiles) stay local to that panel's own static/.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFiles embed.FS

func Handler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/assets/", http.FileServer(http.FS(sub)))
}
