package server

import (
	"tsla-remote-mcp/internal/auth"
	"tsla-remote-mcp/internal/mcp"
	"tsla-remote-mcp/internal/tesla"

	"github.com/go-chi/chi/v5"
)

func SetupRoutes(server *Server) *chi.Mux {
	r := chi.NewRouter()
	r.Use(LoggerMiddleware(server.Logger))
	r.Get(tesla.PartnerPublicKeyPath, tesla.PartnerPublicKeyHandler())
	oauth := auth.NewHandler(server.TeslaAuth, server.Logger)
	r.Handle("/mcp", mcp.NewHandler(server.TeslaClient))
	r.Get("/", oauth.Home())
	r.Route("/auth", func(r chi.Router) {
		r.Get("/tsla", oauth.Start())
		r.Get("/callback", oauth.Callback())
		r.Get("/success", oauth.Home())
	})

	return r
}
