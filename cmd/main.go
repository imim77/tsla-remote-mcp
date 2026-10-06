package main

import (
	"log"
	"net/http"

	"tsla-remote-mcp/internal/server"
)

func main() {
	s := server.NewServer()
	s.Logger.Info("starting HTTP server", "address", ":8080")
	log.Fatal(http.ListenAndServe(":8080", s.Handler()))
}
