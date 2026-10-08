package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type vehicleReaderFunc func(context.Context) (json.RawMessage, error)

func (f vehicleReaderFunc) ListVehicles(ctx context.Context) (json.RawMessage, error) {
	return f(ctx)
}

func TestListVehiclesTool(t *testing.T) {
	const vehicles = `{"response":[{"vin":"TESTVIN"}]}`
	for _, failed := range []bool{false, true} {
		name := "success"
		if failed {
			name = "Tesla error"
		}
		t.Run(name, func(t *testing.T) {
			called := false
			handler := NewHandler(vehicleReaderFunc(func(ctx context.Context) (json.RawMessage, error) {
				called = true
				if failed {
					return nil, errors.New("Tesla unavailable")
				}
				return json.RawMessage(vehicles), nil
			}))
			request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_vehicles","arguments":{}}}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			request.Header.Set("MCP-Protocol-Version", "2025-11-25")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK || !called {
				t.Fatalf("tool was not dispatched: HTTP %d, %s", response.Code, response.Body.String())
			}
			var message struct {
				Error  json.RawMessage
				Result struct {
					IsError bool
					Content []struct{ Type, Text string }
				}
			}
			if err := json.Unmarshal(response.Body.Bytes(), &message); err != nil {
				t.Fatal(err)
			}
			if len(message.Error) != 0 || message.Result.IsError != failed || len(message.Result.Content) != 1 {
				t.Fatalf("unexpected MCP response: %s", response.Body.String())
			}
			want := vehicles
			if failed {
				want = "Tesla unavailable"
			}
			if message.Result.Content[0].Type != "text" || message.Result.Content[0].Text != want {
				t.Fatalf("tool content = %+v", message.Result.Content)
			}
		})
	}
}
