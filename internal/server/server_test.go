package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

type tokenTransportFunc func(*http.Request) (*http.Response, error)

func (f tokenTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestOAuthCallbackRedirectsToSuccessPage(t *testing.T) {
	t.Setenv("TESLA_AUDIENCE", "https://fleet-api.example.com")
	client := &http.Client{Transport: tokenTransportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("token request method = %s, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse token request: %v", err)
			return nil, err
		}
		if r.Form.Get("code") != "test-code" || r.Form.Get("audience") != "https://fleet-api.example.com" {
			t.Error("token request is missing the expected code or audience")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"test-access-token","refresh_token":"test-refresh-token","token_type":"Bearer","expires_in":3600}`)),
		}, nil
	})}

	s := NewServer()
	s.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	s.OAuthConfig = &oauth2.Config{
		ClientID: "test-client", ClientSecret: "test-secret",
		Endpoint: oauth2.Endpoint{TokenURL: "https://tesla.example/token", AuthStyle: oauth2.AuthStyleInParams},
	}
	s.ChiMultiplexer = SetupRoutes(s)
	request := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code&state=test-state", nil)
	request = request.WithContext(context.WithValue(request.Context(), oauth2.HTTPClient, client))
	request.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "test-state"})
	response := httptest.NewRecorder()
	s.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("callback status = %d, want 303; body: %s", response.Code, response.Body.String())
	}
	saved, err := s.TokenStore.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() after callback: %v", err)
	}
	if saved.AccessToken != "test-access-token" || saved.RefreshToken != "test-refresh-token" || saved.Expiry.IsZero() {
		t.Error("callback did not store the received tokens and expiry")
	}
	if location := response.Header().Get("Location"); location != "/auth/success" {
		t.Fatalf("callback Location = %q, want /auth/success", location)
	}
	result := response.Result()
	defer result.Body.Close()
	clearedState := false
	for _, cookie := range result.Cookies() {
		if cookie.Name == oauthStateCookie && cookie.MaxAge < 0 {
			clearedState = true
		}
	}
	if !clearedState {
		t.Error("callback did not clear the OAuth state cookie")
	}

	success := httptest.NewRecorder()
	s.Handler().ServeHTTP(success, httptest.NewRequest(http.MethodGet, response.Header().Get("Location"), nil))
	if success.Code != http.StatusOK {
		t.Fatalf("success page status = %d, want 200", success.Code)
	}
	if !strings.HasPrefix(success.Header().Get("Content-Type"), "text/html") || !strings.Contains(success.Body.String(), "Autorizacija s Teslom uspješno je završena.") {
		t.Error("success page did not return an HTML confirmation")
	}
}

type failingTokenStore struct{}

func (failingTokenStore) Save(context.Context, *oauth2.Token) error {
	return errors.New("storage unavailable")
}

func (failingTokenStore) Load(context.Context) (*oauth2.Token, error) {
	return nil, errors.New("storage unavailable")
}

func TestOAuthCallbackDoesNotRedirectWhenSavingFails(t *testing.T) {
	client := &http.Client{Transport: tokenTransportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"test-access-token","token_type":"Bearer"}`)),
		}, nil
	})}
	s := &Server{
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		TokenStore: failingTokenStore{},
		OAuthConfig: &oauth2.Config{
			Endpoint: oauth2.Endpoint{TokenURL: "https://tesla.example/token", AuthStyle: oauth2.AuthStyleInParams},
		},
	}
	request := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code&state=test-state", nil)
	request = request.WithContext(context.WithValue(request.Context(), oauth2.HTTPClient, client))
	request.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "test-state"})
	response := httptest.NewRecorder()
	s.OAuthCallback().ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("callback status = %d, want 500", response.Code)
	}
	if location := response.Header().Get("Location"); location != "" {
		t.Fatalf("callback redirected despite failed save: %q", location)
	}
}
