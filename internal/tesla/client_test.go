package tesla

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	"tsla-remote-mcp/internal/auth"
	"tsla-remote-mcp/internal/store"

	"golang.org/x/oauth2"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestListVehicles(t *testing.T) {
	const vehicles = `{"response":[{"id":123,"vin":"TESTVIN","display_name":"My Tesla"}],"count":1}`
	tests := []struct {
		name        string
		token       *oauth2.Token
		status      int
		body        string
		wantError   string
		wantRequest bool
	}{
		{name: "vehicles", token: &oauth2.Token{AccessToken: "test-token"}, status: 200, body: vehicles, wantRequest: true},
		{name: "missing token", wantError: "connect your Tesla account"},
		{name: "expired token", token: &oauth2.Token{AccessToken: "test-token", Expiry: time.Now().Add(-time.Hour)}, wantError: "expired"},
		{name: "Tesla error", token: &oauth2.Token{AccessToken: "test-token"}, status: 503, wantError: "HTTP 503", wantRequest: true},
		{name: "invalid JSON", token: &oauth2.Token{AccessToken: "test-token"}, status: 200, body: "not JSON", wantError: "invalid", wantRequest: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens := store.NewInMemoryStore()
			if test.token != nil {
				if err := tokens.Save(context.Background(), test.token); err != nil {
					t.Fatal(err)
				}
			}
			client := NewClient("https://fleet.example.com", auth.NewService(&oauth2.Config{}, "https://fleet.example.com", tokens))
			called := false
			client.httpClient.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
				called = true
				if r.Method != http.MethodGet || r.URL.String() != "https://fleet.example.com/api/1/vehicles" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("missing Bearer token")
				}
				return &http.Response{
					StatusCode: test.status,
					Header:     http.Header{"Content-Type": {"application/json"}},
					Body:       io.NopCloser(strings.NewReader(test.body)),
				}, nil
			})
			got, err := client.ListVehicles(context.Background())
			if called != test.wantRequest {
				t.Errorf("HTTP request sent = %v, want %v", called, test.wantRequest)
			}
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil || string(got) != vehicles {
				t.Fatalf("ListVehicles() = %s, %v", got, err)
			}
		})
	}
}

func TestListVehiclesUsesRefreshedCredentials(t *testing.T) {
	tokens := store.NewInMemoryStore()
	if err := tokens.Save(context.Background(), &oauth2.Token{AccessToken: "expired", RefreshToken: "old-refresh", Expiry: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	service := auth.NewService(&oauth2.Config{
		ClientID: "test-client", ClientSecret: "test-secret",
		Endpoint: oauth2.Endpoint{TokenURL: "https://auth.example.com/token", AuthStyle: oauth2.AuthStyleInParams},
	}, "https://fleet.example.com", tokens)
	client := NewClient("https://fleet.example.com", service)
	refreshes, vehicleCalls := 0, 0
	transport := transportFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"response":[{"vin":"TESTVIN"}]}`
		if r.URL.Path == "/token" {
			refreshes++
			body = `{"access_token":"renewed","refresh_token":"rotated","token_type":"Bearer","expires_in":3600}`
		} else {
			vehicleCalls++
			if r.Header.Get("Authorization") != "Bearer renewed" {
				t.Error("Fleet API received an old token")
			}
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	client.httpClient.Transport = transport
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Transport: transport})
	for range 2 {
		if _, err := client.ListVehicles(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if refreshes != 1 || vehicleCalls != 2 {
		t.Fatalf("refreshes = %d, vehicle calls = %d", refreshes, vehicleCalls)
	}
	saved, err := tokens.Load(context.Background())
	if err != nil || saved.RefreshToken != "rotated" {
		t.Fatal("rotated refresh token was not retained")
	}
}
