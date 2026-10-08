package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
	"tsla-remote-mcp/internal/store"

	"golang.org/x/oauth2"
)

var (
	ErrNotConnected      = errors.New("connect your Tesla account through /auth/tsla first")
	ErrReconnectRequired = errors.New("Tesla authorization has expired or been revoked; reconnect through /auth/tsla")
	ErrTokenStorage      = errors.New("could not save Tesla tokens")
)

type Service struct {
	config            *oauth2.Config
	audience          string
	tokens            store.Repository
	gate              chan struct{}
	reconnectRequired bool
	httpClient        *http.Client
}

func NewService(config *oauth2.Config, audience string, tokens store.Repository) *Service {
	return &Service{
		config: config, audience: audience, tokens: tokens,
		gate:       make(chan struct{}, 1),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// One operation at a time prevents concurrent use of a rotating refresh token.
func (s *Service) lock(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case s.gate <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) oauthContext(ctx context.Context) context.Context {
	if ctx.Value(oauth2.HTTPClient) != nil {
		return ctx
	}
	return context.WithValue(ctx, oauth2.HTTPClient, s.httpClient)
}

func (s *Service) AuthorizationURL(state string) (string, error) {
	if s.config.ClientID == "" || s.config.ClientSecret == "" {
		return "", errors.New("TESLA_CLIENT_ID and TESLA_CLIENT_SECRET must be configured")
	}
	redirect, err := url.Parse(s.config.RedirectURL)
	if err != nil || redirect.Host == "" || (redirect.Scheme != "https" && redirect.Scheme != "http") {
		return "", errors.New("DOMAIN_SERVICE must be an absolute HTTP or HTTPS URL")
	}
	audience, err := url.Parse(s.audience)
	if err != nil || audience.Scheme != "https" || audience.Host == "" {
		return "", errors.New("TESLA_AUDIENCE must be a Fleet API HTTPS base URL")
	}
	return s.config.AuthCodeURL(state), nil
}

func (s *Service) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	if _, err := s.AuthorizationURL(""); err != nil {
		return nil, err
	}
	if err := s.lock(ctx); err != nil {
		return nil, err
	}
	defer func() { <-s.gate }()
	token, err := s.config.Exchange(s.oauthContext(ctx), code, oauth2.SetAuthURLParam("audience", s.audience))
	if err != nil {
		return nil, fmt.Errorf("exchange Tesla authorization code: %w", err)
	}
	if err := s.save(ctx, token); err != nil {
		return nil, err
	}
	s.reconnectRequired = false
	return token, nil
}

func (s *Service) Token(ctx context.Context) (*oauth2.Token, error) {
	if err := s.lock(ctx); err != nil {
		return nil, err
	}
	defer func() { <-s.gate }()
	if s.reconnectRequired {
		return nil, ErrReconnectRequired
	}
	token, err := s.tokens.Load(ctx)
	if errors.Is(err, store.ErrTokenNotFound) {
		return nil, ErrNotConnected
	}
	if err != nil {
		return nil, fmt.Errorf("load Tesla tokens: %w", err)
	}
	if token.Valid() {
		return token, nil
	}
	if token == nil || token.RefreshToken == "" {
		return nil, ErrReconnectRequired
	}
	refreshed, err := s.config.TokenSource(s.oauthContext(ctx), token).Token()
	if err != nil {
		var rejected *oauth2.RetrieveError
		if errors.As(err, &rejected) && (rejected.ErrorCode == "invalid_grant" || rejected.ErrorCode == "login_required" || (rejected.Response != nil && rejected.Response.StatusCode == http.StatusUnauthorized)) {
			s.reconnectRequired = true
			return nil, ErrReconnectRequired
		}
		return nil, fmt.Errorf("refresh Tesla token: %w", err)
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = token.RefreshToken
	}
	if err := s.save(ctx, refreshed); err != nil {
		return nil, err
	}
	return refreshed, nil
}

func (s *Service) save(ctx context.Context, token *oauth2.Token) error {
	// Preserve rotated credentials even if the caller disconnects after exchange.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.tokens.Save(ctx, token); err != nil {
		return fmt.Errorf("%w: %w", ErrTokenStorage, err)
	}
	return nil
}

func (s *Service) Connected(ctx context.Context) (bool, error) {
	if err := s.lock(ctx); err != nil {
		return false, err
	}
	defer func() { <-s.gate }()
	token, err := s.tokens.Load(ctx)
	if errors.Is(err, store.ErrTokenNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !s.reconnectRequired && token != nil && token.AccessToken != "" && (token.Valid() || token.RefreshToken != ""), nil
}
