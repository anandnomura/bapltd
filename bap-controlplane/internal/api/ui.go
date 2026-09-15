package api

import (
	"encoding/json"
	"net/http"
	"os"
)

func (s *Server) registerControlPlaneRoutes() {
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"service":      "bapcontrolplane",
			"status":       "ok",
			"inspector":    "/inspector",
			"inspector_v2": "/inspector_v2",
			"dashboard":    "not hosted by this service; start bapdashboard separately",
		})
	})

	s.mux.HandleFunc("/inspector", s.handleInspectorHTML("inspector.html", "BAP Activity Inspector"))
	s.mux.HandleFunc("/inspector.html", s.handleInspectorHTML("inspector.html", "BAP Activity Inspector"))
	s.mux.HandleFunc("/inspector_v2", s.handleInspectorHTML("inspector_v2.html", "BAP Inspector V2 - Executive Cockpit"))
	s.mux.HandleFunc("/inspector_v2.html", s.handleInspectorHTML("inspector_v2.html", "BAP Inspector V2 - Executive Cockpit"))
}

func (s *Server) handleInspectorHTML(filename string, title string) http.HandlerFunc {
	candidates := []string{
		filename,
		"../" + filename,
		"../../" + filename,
		"bap-controlplane/" + filename,
		"../bap-controlplane/" + filename,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		for _, candidate := range candidates {
			if data, err := os.ReadFile(candidate); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				if r.Method != http.MethodHead {
					_, _ = w.Write(data)
				}
				return
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>` + title + `</title></head><body style="font-family:sans-serif;background:#0f172a;color:#fff;padding:2rem;"><h2>` + title + `</h2><p>File <code>` + filename + `</code> was not found in working directories.</p></body></html>`))
	}
}
