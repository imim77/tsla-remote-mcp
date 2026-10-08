package server

import (
	"net/http"
	"os"
	"tsla-remote-mcp/internal/mcp"
	"tsla-remote-mcp/internal/tesla"

	"github.com/go-chi/chi/v5"
)

func SetupRoutes(server *Server) *chi.Mux {
	r := chi.NewRouter()
	r.Use(LoggerMiddleware(server.Logger))
	r.Handle("/mcp", mcp.NewHandler(tesla.NewClient(os.Getenv("TESLA_AUDIENCE"), server.TokenStore)))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/auth/tsla", http.StatusFound)
	})

	r.Group(func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Get("/tsla", server.InitializeTeslaAuth())
			r.Get("/callback", server.OAuthCallback())
			r.Get("/success", server.OAuthSuccess())
		})
	})

	return r
}
