package mcp

import (
	"context"
	"encoding/json"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type VehicleReader interface {
	ListVehicles(context.Context) (json.RawMessage, error)
}

func (s *Server) RetrieveBasicVehicleData(ctx context.Context, req *sdk.CallToolRequest, args struct{}) (*sdk.CallToolResult, any, error) {
	if s.vehicles == nil {
		return toolError("Tesla client is not configured"), nil, nil
	}
	vehicles, err := s.vehicles.ListVehicles(ctx)
	if err != nil {
		return toolError(err.Error()), nil, nil
	}
	return &sdk.CallToolResult{
		Content: []sdk.Content{&sdk.TextContent{Text: string(vehicles)}},
	}, nil, nil
}

func toolError(message string) *sdk.CallToolResult {
	return &sdk.CallToolResult{
		IsError: true,
		Content: []sdk.Content{&sdk.TextContent{Text: message}},
	}
}
