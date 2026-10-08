package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"tsla-remote-mcp/internal/server"
)

func main() {
	s := server.NewServer()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	s.Logger.Info("starting HTTP server", "address", ":"+port)
	httpServer := &http.Server{Addr: ":" + port, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(httpServer.ListenAndServe())
}
