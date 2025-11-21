package main

import (
	"limedb-go/internal/config"
	"limedb-go/internal/node"
	"limedb-go/internal/server"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Load configuration
	cfg := config.Load()

	log.Printf("Starting LimeDB Node on port %d", cfg.ServerPort)
	log.Printf("Peers: %v", cfg.Peers)

	// Initialize Node Service
	svc := node.New(
		cfg.ServerPort,
		cfg.VirtualNodes,
		cfg.Peers,
	)

	// Initialize HTTP Server
	srv := server.New(svc, cfg.ServerPort)

	// Start Server in a goroutine
	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("Server stopped: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	if err := srv.Shutdown(); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
