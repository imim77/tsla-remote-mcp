package server

import (
	"log/slog"
	"os"
	"tsla-remote-mcp/internal/auth"

	"golang.org/x/oauth2"
)

type Server struct {
	OAuthConfig *oauth2.Config
	Logger      *slog.Logger
}

func NewServer() *Server {
	return &Server{
		OAuthConfig: auth.InitializeOAuthConfig(),
		Logger:      slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})),
	}
}
