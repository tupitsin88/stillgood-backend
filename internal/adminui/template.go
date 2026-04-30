package adminui

import (
	"embed"
	"html/template"
)

//go:embed assets/admin.css templates/*.html
var adminFS embed.FS

func parseTemplates() *template.Template {
	return template.Must(template.ParseFS(adminFS, "templates/*.html"))
}
