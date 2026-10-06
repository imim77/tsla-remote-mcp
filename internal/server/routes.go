package server

import (
	"github.com/go-chi/chi/v5"
)

func SetupRoutes(server *Server) *chi.Mux {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Get("/callback", server.OAuthCallback())
		})
	})

	return r
}
