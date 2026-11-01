package ui

import (
	"io/fs"
	"net/http"
)

// NewHandler serves the single-screen UI, its static assets, and the HTMX endpoints.
// pickDirectory opens the native directory chooser and returns the selected path, or an
// empty string if the user cancelled.
func NewHandler(static fs.FS, pickDirectory func() (string, error)) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Page("").Render(r.Context(), w)
	})

	mux.HandleFunc("POST /pick-directory", func(w http.ResponseWriter, r *http.Request) {
		dir, err := pickDirectory()
		if err != nil || dir == "" {
			// 204 tells HTMX to leave the current field unchanged.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = DirectoryField(dir).Render(r.Context(), w)
	})

	return mux
}
