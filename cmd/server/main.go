package main

import (
	"context"
	"fmt"
	"limedb-go/internal/config"
	"limedb-go/internal/node"
	"limedb-go/internal/server"
	"limedb-go/internal/telemetry"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Configure logging with timestamps
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Load configuration
	cfg := config.Load()

	// Initialize Telemetry
	shutdownTelemetry, err := telemetry.Init("limedb-node", "1.0.0", cfg.OtelEndpoint)
	if err != nil {
		log.Fatalf("Failed to initialize telemetry: %v", err)
	}
	defer func() {
		if err := shutdownTelemetry(context.Background()); err != nil {
			log.Printf("Error shutting down telemetry: %v", err)
		}
	}()

	// Display startup banner
	printStartupBanner()

	// Log configuration details
	log.Printf("🚀 Starting LimeDB Node")
	log.Printf("   Node URL: %s", cfg.NodeUrl)
	log.Printf("   Server Port: %d", cfg.ServerPort)
	log.Printf("   Virtual Nodes: %d", cfg.VirtualNodes)

	if len(cfg.Peers) > 0 {
		log.Printf("   Cluster Peers: %d nodes", len(cfg.Peers))
		for i, peer := range cfg.Peers {
			log.Printf("     [%d] %s", i+1, peer)
		}
	} else {
		log.Printf("   Cluster Mode: Single node (no peers configured)")
	}

	// Initialize Node Service
	log.Printf("🔧 Initializing node service...")
	svc := node.New(
		cfg.NodeUrl,
		cfg.VirtualNodes,
		cfg.Peers,
	)

	// Initialize HTTP Server
	log.Printf("🌐 Initializing HTTP server...")
	srv := server.New(svc, cfg.ServerPort)

	// Start Server in a goroutine
	serverStarted := make(chan bool, 1)
	go func() {
		log.Printf("✅ LimeDB Node ready and listening on port %d", cfg.ServerPort)
		log.Printf("   Health endpoint: http://localhost:%d/api/v1/health", cfg.ServerPort)
		log.Printf("   API endpoint: http://localhost:%d/api/v1/", cfg.ServerPort)
		serverStarted <- true

		if err := srv.Start(); err != nil {
			log.Printf("❌ Server error: %v", err)
		}
	}()

	// Wait for server to start
	<-serverStarted

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("🛑 Shutdown signal received, gracefully stopping server...")
	if err := srv.Shutdown(); err != nil {
		log.Printf("⚠️  Server forced to shutdown: %v", err)
	} else {
		log.Printf("✅ Server shutdown completed successfully")
	}

	log.Printf("👋 LimeDB Node exited")
}

func printStartupBanner() {
	banner := `
╔══════════════════════════════════════╗
║             🍋 LimeDB                ║  
║        High-Performance K-V Store    ║
║          Distributed & Fast          ║
╚══════════════════════════════════════╝`
	fmt.Println(banner)
}
