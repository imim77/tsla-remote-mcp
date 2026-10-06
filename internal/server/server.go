package server

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"tsla-remote-mcp/internal/auth"

	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"
)

type Server struct {
	OAuthConfig    *oauth2.Config
	Logger         *slog.Logger
	ChiMultiplexer *chi.Mux
}

func NewServer() *Server {
	s := &Server{
		OAuthConfig: auth.InitializeOAuthConfig(),
		Logger:      slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})),
	}
	s.ChiMultiplexer = SetupRoutes(s)
	return s
}

func (s *Server) OAuthCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		_, err := s.OAuthConfig.Exchange(r.Context(), code)
		if err != nil {
			log.Print(err)
			http.Error(w, "OAuth exchange failed", http.StatusBadGateway)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) Handler() http.Handler {
	return s.ChiMultiplexer
}
