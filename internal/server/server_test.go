package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"tsla-remote-mcp/internal/auth"
	"tsla-remote-mcp/internal/store"
	"tsla-remote-mcp/internal/tesla"

	"golang.org/x/oauth2"
)

func TestServerAvailableBeforeTeslaConnection(t *testing.T) {
	t.Setenv("TESLA_AUDIENCE", "https://fleet.example.com")
	s := NewServer()
	s.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	s.ChiMultiplexer = SetupRoutes(s)
	home := httptest.NewRecorder()
	s.Handler().ServeHTTP(home, httptest.NewRequest(http.MethodGet, "/", nil))
	if home.Code != http.StatusOK || home.Header().Get("Location") != "" || !strings.Contains(home.Body.String(), "Poveži Tesla račun") {
		t.Fatalf("home should show connection status, got %d %s", home.Code, home.Body.String())
	}
	for _, name := range []string{"ping", "list_vehicles", "vehicle_data"} {
		t.Run(name, func(t *testing.T) {
			arguments := `{}`
			if name == "vehicle_data" {
				arguments = `{"vin":"TESTVIN"}`
			}
			request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+name+`","arguments":`+arguments+`}}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			request.Header.Set("MCP-Protocol-Version", "2025-11-25")
			response := httptest.NewRecorder()
			s.Handler().ServeHTTP(response, request)
			var message struct {
				Result struct {
					IsError bool
					Content []struct{ Text string }
				}
			}
			if err := json.Unmarshal(response.Body.Bytes(), &message); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || len(message.Result.Content) != 1 {
				t.Fatalf("unexpected MCP response: %s", response.Body.String())
			}
			if name == "ping" {
				if message.Result.IsError || message.Result.Content[0].Text != "pong" {
					t.Fatal("ping should work without a Tesla account")
				}
			} else if !message.Result.IsError || !strings.Contains(message.Result.Content[0].Text, "/auth/tsla") {
				t.Fatal("vehicle tool should request Tesla connection")
			}
		})
	}
}

func TestHomeShowsConnectedAccountWithoutStartingLogin(t *testing.T) {
	tokens := store.NewInMemoryStore()
	if err := tokens.Save(context.Background(), &oauth2.Token{AccessToken: "test-token"}); err != nil {
		t.Fatal(err)
	}
	service := auth.NewService(&oauth2.Config{}, "https://fleet.example.com", tokens)
	s := &Server{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), TeslaAuth: service,
		TeslaClient: tesla.NewClient("https://fleet.example.com", service),
	}
	s.ChiMultiplexer = SetupRoutes(s)
	for _, path := range []string{"/", "/auth/success"} {
		response := httptest.NewRecorder()
		s.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Tesla račun je povezan") || response.Header().Get("Location") != "" {
			t.Fatalf("%s should show connected account: %d %s", path, response.Code, response.Body.String())
		}
	}
}
