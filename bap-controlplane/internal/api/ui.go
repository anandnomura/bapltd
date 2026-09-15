package api

import (
	"embed"
	"io/fs"
	"net/http"
)

// Build dashboard/ before compiling Go when frontend sources have changed.
// Checked-in build output keeps normal Go builds independent of Node.
//go:embed web/dashboard-build web/admin-client.js
var dashboardAssets embed.FS

func (s *Server) registerDashboardRoutes() {
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/dashboard/", http.StatusTemporaryRedirect)
	})
	s.mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard/", http.StatusTemporaryRedirect)
	})
	assets, _ := fs.Sub(dashboardAssets, "web/dashboard-build")
	files := http.StripPrefix("/dashboard/", http.FileServer(http.FS(assets)))
	s.mux.Handle("/dashboard/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		files.ServeHTTP(w, r)
	}))
	s.mux.HandleFunc("/assets/admin-client.js", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		data, _ := dashboardAssets.ReadFile("web/admin-client.js")
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		if r.Method != "HEAD" {
			_, _ = w.Write(data)
		}
	})
}
