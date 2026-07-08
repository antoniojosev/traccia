package admin

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templatesFS embed.FS

func parseTemplates() *template.Template {
	return template.Must(template.ParseFS(templatesFS, "templates/*.html"))
}
