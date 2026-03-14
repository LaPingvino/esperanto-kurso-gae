package handler

import "io"

// Renderer is implemented by the per-page template set built in main.go.
// It has the same ExecuteTemplate signature as *html/template.Template,
// allowing handlers to remain unaware of the per-page map internals.
type Renderer interface {
	ExecuteTemplate(w io.Writer, name string, data interface{}) error
}
