package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const stateCookie = "tesla_oauth_state"

type Handler struct {
	service       *Service
	logger        *slog.Logger
	secureCookies bool
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	redirect, _ := url.Parse(service.config.RedirectURL)
	return &Handler{service: service, logger: logger, secureCookies: redirect != nil && redirect.Scheme == "https"}
}

var connectionPage = template.Must(template.New("connection").Parse(`<!doctype html>
<html lang="hr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Tesla MCP</title></head>
<body><h1>Tesla MCP</h1>
{{if .}}<p>Tesla račun je povezan. MCP alati su spremni za korištenje.</p><a href="/auth/tsla">Ponovno poveži Teslu</a>
{{else}}<p>Poveži Tesla račun kako bi mogao koristiti Tesla MCP alate.</p><a href="/auth/tsla">Poveži Teslu</a>{{end}}
<p>MCP endpoint: <code>/mcp</code></p><p>Povezivanje vrijedi do ponovnog pokretanja aplikacije.</p></body></html>`))

func (h *Handler) Home() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connected, err := h.service.Connected(r.Context())
		if err != nil {
			h.logger.Error("Failed to read Tesla connection status", "error", err)
			http.Error(w, "Could not read Tesla connection status", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := connectionPage.Execute(w, connected); err != nil {
			h.logger.Error("Failed to render connection page", "error", err)
		}
	}
}

func (h *Handler) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: stateCookie, Value: value, Path: "/auth", MaxAge: maxAge,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.secureCookies}
}

func (h *Handler) Start() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stateBytes := make([]byte, 32)
		if _, err := rand.Read(stateBytes); err != nil {
			h.logger.Error("Failed to generate OAuth state", "error", err)
			http.Error(w, "Could not start authorization", http.StatusInternalServerError)
			return
		}
		state := hex.EncodeToString(stateBytes)
		authorizationURL, err := h.service.AuthorizationURL(state)
		if err != nil {
			h.logger.Error("Tesla OAuth configuration is invalid", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.SetCookie(w, h.cookie(state, 600))
		http.Redirect(w, r, authorizationURL, http.StatusFound)
	}
}

func (h *Handler) Callback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		cookie, err := r.Cookie(stateCookie)
		state := r.URL.Query().Get("state")
		if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookie.Value)) != 1 {
			http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
			return
		}
		http.SetCookie(w, h.cookie("", -1))
		if r.URL.Query().Get("error") != "" {
			http.Error(w, "Tesla authorization failed", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing authorization code", http.StatusBadRequest)
			return
		}
		token, err := h.service.Exchange(r.Context(), code)
		if err != nil {
			h.logger.Error("Tesla authorization failed", "error", err)
			status := http.StatusBadGateway
			if errors.Is(err, ErrTokenStorage) {
				status = http.StatusInternalServerError
			}
			http.Error(w, "Could not connect Tesla account", status)
			return
		}
		expiresIn := "unknown"
		if !token.Expiry.IsZero() {
			expiresIn = time.Until(token.Expiry).Round(time.Second).String()
		}
		h.logger.Info("OAuth tokens received", "access_token_prefix", firstSeven(token.AccessToken),
			"refresh_token_prefix", firstSeven(token.RefreshToken), "access_expires_in", expiresIn)
		http.Redirect(w, r, "/auth/success", http.StatusSeeOther)
	}
}

func firstSeven(value string) string {
	if len(value) < 7 {
		return "<short or empty>"
	}
	return value[:7]
}
