package server

import (
	"log/slog"
	"net/http"
	"os"
	"tsla-remote-mcp/internal/auth"
	"tsla-remote-mcp/internal/store"
	"tsla-remote-mcp/internal/tesla"

	"github.com/go-chi/chi/v5"
)

type Server struct {
	Logger         *slog.Logger
	ChiMultiplexer *chi.Mux
	TeslaAuth      *auth.Service
	TeslaClient    *tesla.Client
}

func NewServer() *Server {
	audience := os.Getenv("TESLA_AUDIENCE")
	tokens := store.NewInMemoryStore()
	teslaAuth := auth.NewService(auth.InitializeOAuthConfig(), audience, tokens)
	s := &Server{
		Logger:      slog.New(slog.NewTextHandler(os.Stdout, nil)),
		TeslaAuth:   teslaAuth,
		TeslaClient: tesla.NewClient(audience, teslaAuth),
	}
	s.ChiMultiplexer = SetupRoutes(s)
	return s
}

func (s *Server) Handler() http.Handler {
	return s.ChiMultiplexer
}
