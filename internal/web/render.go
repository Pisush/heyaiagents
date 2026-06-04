package web

import (
	"net/http"

	"github.com/a-h/templ"
)

// render writes a templ component as an HTML response, mapping a render failure
// to a 500.
func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
