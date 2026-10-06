package mcp

import (
	"context"
	"net/http"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ServerName    = "tsla-remote-mcp"
	ServerVersion = "0.1.0"
)

type Server struct {
	srv *sdk.Server
}

func NewServer() *Server {
	server := &Server{
		srv: sdk.NewServer(&sdk.Implementation{
			Name:    ServerName,
			Version: ServerVersion,
		}, nil),
	}
	server.registerTools()
	return server
}

func (mcp *Server) registerTools() {
	sdk.AddTool(mcp.srv, &sdk.Tool{
		Name:        "ping",
		Description: "Check that the MCP server is responding.",
	}, func(ctx context.Context, req *sdk.CallToolRequest, args struct{}) (*sdk.CallToolResult, any, error) {
		return &sdk.CallToolResult{
			Content: []sdk.Content{&sdk.TextContent{Text: "pong"}},
		}, nil, nil
	})
}

func NewHandler() http.Handler {
	server := NewServer()
	return sdk.NewStreamableHTTPHandler(func(r *http.Request) *sdk.Server {
		return server.srv
	}, &sdk.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
}
