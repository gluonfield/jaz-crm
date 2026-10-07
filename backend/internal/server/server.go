// Package server is the HTTP process shell: routing, CORS and the web app.
package server

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/authapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/connectapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/logosapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/mcpapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/pageimagesapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/webhooks"
	"github.com/gluonfield/jaz-tasks/httpx"
)

// WebDir holds the built web app; empty serves a sign-in page instead.
type WebDir string

func New(authn *authapi.Handler, agents *mcpapi.Handler, connect *connectapi.Handler, hooks *webhooks.Handler, logos *logosapi.Handler, images *pageimagesapi.Handler, web WebDir) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/auth/", authn)
	mux.Handle("GET /login", authn)
	mux.Handle("/oauth/", cors(authn))
	mux.Handle("/.well-known/", cors(authn))
	mux.Handle("/mcp", cors(agents.MCP))
	mux.HandleFunc("GET /favicon.svg", mcpapi.Favicon)
	mux.HandleFunc("GET /favicon.ico", mcpapi.Favicon)
	mux.Handle("POST /api/tools/{tool}", authn.Session(agents.API))
	mux.Handle("GET /connections/google/start", authn.Session(http.HandlerFunc(connect.Start)))
	mux.Handle("GET /connections/google/callback", authn.Session(http.HandlerFunc(connect.Callback)))
	mux.HandleFunc("POST /webhooks/google/gmail", hooks.Gmail)
	mux.HandleFunc("POST /webhooks/google/calendar", hooks.Calendar)
	mux.HandleFunc("POST /webhooks/interactions", hooks.Interactions)
	mux.Handle("GET /logos/{token}", logos)
	mux.Handle("GET /page-icons/{token}", images)
	if web == "" {
		mux.Handle("GET /{$}", authn)
	} else {
		mux.Handle("/", spa(string(web)))
	}
	return httpx.Compress(mux)
}

// cors lets browser clients on other origins call the API: credentials
// travel in a header, never in cookies, so any origin is safe.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Mcp-Session-Id, Mcp-Protocol-Version")
			w.Header().Set("Access-Control-Expose-Headers", "WWW-Authenticate, Mcp-Session-Id")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// spa serves the built app, falling back to index.html for client routes.
// Other methods fail, so a client posting to a wrong URL gets an error
// rather than the page.
func spa(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "not found; MCP is served at /mcp", http.StatusNotFound)
			return
		}
		if info, err := os.Stat(filepath.Join(dir, filepath.Clean("/"+r.URL.Path))); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	})
}
