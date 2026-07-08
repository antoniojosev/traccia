package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFiles embed.FS

func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		// static/ is embedded at compile time — this can only fail if the
		// embed directive itself is broken, which build would already catch.
		panic(err)
	}
	return http.StripPrefix("/dashboard/static/", http.FileServer(http.FS(sub)))
}
