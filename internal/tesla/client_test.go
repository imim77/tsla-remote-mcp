package tesla

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
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
		{name: "missing token", wantError: "sign in to Tesla"},
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
			client := NewClient("https://fleet.example.com", tokens)
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
