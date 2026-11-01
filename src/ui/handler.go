package ui

import (
	"io/fs"
	"net/http"
)

// NewHandler serves the single-screen UI, its static assets, the HTMX endpoints, and the SSE
// stream. pickDirectory opens the native directory chooser and returns the selected path, or an
// empty string if the user cancelled.
func NewHandler(static fs.FS, pickDirectory func() (string, error)) http.Handler {
	screen := NewScreen()
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Page(screen.Snapshot()).Render(r.Context(), w)
	})

	mux.HandleFunc("POST /pick-directory", func(w http.ResponseWriter, r *http.Request) {
		dir, err := pickDirectory()
		if err != nil || dir == "" {
			// 204 tells HTMX to leave the current panel unchanged.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		screen.SetDirectory(dir)
		renderPanel(w, r, screen)
	})

	mux.HandleFunc("POST /start", func(w http.ResponseWriter, r *http.Request) {
		screen.Start()
		renderPanel(w, r, screen)
	})

	mux.HandleFunc("POST /stop", func(w http.ResponseWriter, r *http.Request) {
		screen.Stop()
		renderPanel(w, r, screen)
	})

	mux.HandleFunc("GET /events", screen.serveEvents)

	return mux
}

func renderPanel(w http.ResponseWriter, r *http.Request, screen *Screen) {
	_ = PanelBody(screen.Snapshot()).Render(r.Context(), w)
}
