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

type vehicleReaderStub struct {
	listVehicles func(context.Context) (json.RawMessage, error)
	vehicleData  func(context.Context, string) (json.RawMessage, error)
}

func (s vehicleReaderStub) ListVehicles(ctx context.Context) (json.RawMessage, error) {
	return s.listVehicles(ctx)
}

func (s vehicleReaderStub) VehicleData(ctx context.Context, vin string) (json.RawMessage, error) {
	return s.vehicleData(ctx, vin)
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
			handler := NewHandler(vehicleReaderStub{listVehicles: func(ctx context.Context) (json.RawMessage, error) {
				called = true
				if failed {
					return nil, errors.New("Tesla unavailable")
				}
				return json.RawMessage(vehicles), nil
			}})
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

func TestVehicleDataTool(t *testing.T) {
	const data = `{"response":{"vin":"TESTVIN","charge_state":{"battery_level":80},"future_field":123}}`
	tests := []struct {
		name      string
		arguments string
		client    bool
		upstream  error
		wantText  string
		wantError bool
	}{
		{name: "success", arguments: `{"vin":"TESTVIN"}`, client: true, wantText: data},
		{name: "Tesla error", arguments: `{"vin":"TESTVIN"}`, client: true, upstream: errors.New("Tesla unavailable"), wantText: "Tesla unavailable", wantError: true},
		{name: "unconfigured", arguments: `{"vin":"TESTVIN"}`, wantText: "Tesla client is not configured", wantError: true},
		{name: "missing VIN", arguments: `{}`, client: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			key := struct{}{}
			ctx = context.WithValue(ctx, key, "caller")
			called := false
			var reader VehicleReader
			if test.client {
				reader = vehicleReaderStub{vehicleData: func(gotCtx context.Context, vin string) (json.RawMessage, error) {
					called = true
					if vin != "TESTVIN" || gotCtx.Value(key) != "caller" {
						t.Errorf("VIN or request context was not propagated: %q", vin)
					}
					return json.RawMessage(data), test.upstream
				}}
			}
			handler := NewHandler(reader)
			request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"vehicle_data","arguments":`+test.arguments+`}}`)).WithContext(ctx)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			request.Header.Set("MCP-Protocol-Version", "2025-11-25")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			var message struct {
				Error  *struct{ Code int }
				Result struct {
					IsError bool
					Content []struct{ Type, Text string }
				}
			}
			if err := json.Unmarshal(response.Body.Bytes(), &message); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK {
				t.Fatalf("unexpected HTTP response: %d %s", response.Code, response.Body.String())
			}
			if test.arguments == `{}` {
				if called || message.Error != nil || !message.Result.IsError || len(message.Result.Content) != 1 || !strings.Contains(message.Result.Content[0].Text, "vin") {
					t.Fatalf("missing VIN should be rejected before dispatch: %s", response.Body.String())
				}
				return
			}
			if called != test.client || message.Error != nil || message.Result.IsError != test.wantError || len(message.Result.Content) != 1 {
				t.Fatalf("unexpected MCP response: %s", response.Body.String())
			}
			if content := message.Result.Content[0]; content.Type != "text" || content.Text != test.wantText {
				t.Fatalf("tool content = %+v", content)
			}
		})
	}
}
