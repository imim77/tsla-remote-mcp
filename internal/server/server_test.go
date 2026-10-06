package server

import (
	"context"
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

	s := &Server{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		OAuthConfig: &oauth2.Config{
			ClientID: "test-client", ClientSecret: "test-secret",
			Endpoint: oauth2.Endpoint{TokenURL: "https://tesla.example/token", AuthStyle: oauth2.AuthStyleInParams},
		},
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
