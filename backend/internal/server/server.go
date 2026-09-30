// Package server is the HTTP process shell: routing, auth and CORS.
package server

import (
	"net/http"

	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/authapi"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/mcpapi"
)

func New(authn *authapi.Handler, agents *mcpapi.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("GET /{$}", authn)
	mux.Handle("/auth/", authn)
	mux.Handle("/oauth/", cors(authn))
	mux.Handle("/.well-known/", cors(authn))
	mux.Handle("/mcp", cors(agents.MCP))
	mux.Handle("POST /api/tools/{tool}", authn.Session(agents.API))
	return mux
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
