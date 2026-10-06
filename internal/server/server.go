package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"time"
	"tsla-remote-mcp/internal/auth"
	"tsla-remote-mcp/internal/store"

	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"
)

const oauthStateCookie = "tesla_oauth_state"

type Server struct {
	OAuthConfig    *oauth2.Config
	Logger         *slog.Logger
	ChiMultiplexer *chi.Mux
	TokenStore     store.Repository
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
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		stateCookie, err := r.Cookie(oauthStateCookie)
		state := r.URL.Query().Get("state")
		if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(stateCookie.Value)) != 1 {
			http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: oauthStateCookie, Value: "", Path: "/auth",
			MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode,
			Secure: r.TLS != nil,
		})

		if oauthError := r.URL.Query().Get("error"); oauthError != "" {
			http.Error(w, "Tesla authorization failed", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing authorization code", http.StatusBadRequest)
			return
		}
		token, err := s.OAuthConfig.Exchange(r.Context(), code, oauth2.SetAuthURLParam("audience", os.Getenv("TESLA_AUDIENCE")))
		if err != nil {
			s.Logger.Error("OAuth exchange failed", "error", err)
			http.Error(w, "OAuth exchange failed", http.StatusBadGateway)
			return
		}
		expiresIn := "unknown"
		if !token.Expiry.IsZero() {
			expiresIn = time.Until(token.Expiry).Round(time.Second).String()
		}
		s.Logger.Info("OAuth tokens received",
			"access_token_prefix", firstSeven(token.AccessToken),
			"refresh_token_prefix", firstSeven(token.RefreshToken),
			"access_expires_in", expiresIn,
		)

		http.Redirect(w, r, "/auth/success", http.StatusSeeOther)
	}
}

func (s *Server) OAuthSuccess() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`<!doctype html>
<html lang="hr">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Tesla autorizacija</title></head>
<body><h1>Autorizacija s Teslom uspješno je završena.</h1><p>Možete zatvoriti ovu stranicu.</p></body>
</html>`))
	}
}

func firstSeven(value string) string {
	if len(value) < 7 {
		return "<short or empty>"
	}
	return value[:7]
}

func (s *Server) InitializeTeslaAuth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.OAuthConfig.ClientID == "" {
			http.Error(w, "TESLA_CLIENT_ID is not configured", http.StatusInternalServerError)
			return
		}
		stateBytes := make([]byte, 32)
		if _, err := rand.Read(stateBytes); err != nil {
			s.Logger.Error("Failed to generate OAuth state", "error", err)
			http.Error(w, "Could not start authorization", http.StatusInternalServerError)
			return
		}
		state := hex.EncodeToString(stateBytes)
		http.SetCookie(w, &http.Cookie{
			Name: oauthStateCookie, Value: state, Path: "/auth",
			MaxAge: 600, HttpOnly: true, SameSite: http.SameSiteLaxMode,
			Secure: r.TLS != nil,
		})
		http.Redirect(w, r, s.OAuthConfig.AuthCodeURL(state), http.StatusFound)
	}
}

func (s *Server) Handler() http.Handler {
	return s.ChiMultiplexer
}

func LoggerMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedTime := time.Now()

			logger.Info("request received",
				"method", r.Method,
				"path", r.URL.Path,
			)

			next.ServeHTTP(w, r)

			logger.Info("request complete",
				"method", r.Method,
				"path", r.URL.Path,
				"duration_ms", time.Since(receivedTime).Milliseconds(),
			)
		})
	}
}
