package main

import (
	"context"
	"fmt"
	"limedb-go/internal/config"
	"limedb-go/internal/logger"
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
	shutdownTelemetry, err := telemetry.Init("limedb-node", "1.0.0", cfg.OtelEndpoint, cfg.NodeUrl)
	if err != nil {
		log.Fatalf("Failed to initialize telemetry: %v", err)
	}
	defer func() {
		if err := shutdownTelemetry(context.Background()); err != nil {
			log.Printf("Error shutting down telemetry: %v", err)
		}
	}()

	// Initialize OTEL Logger
	logger.Init(cfg.NodeUrl)

	// Display startup banner
	printStartupBanner()

	// Log configuration details
	logger.Info("🚀 Starting LimeDB Node",
		"node.url", cfg.NodeUrl,
		"server.port", cfg.ServerPort,
		"virtual.nodes", cfg.VirtualNodes,
	)

	if len(cfg.Peers) > 0 {
		logger.Info("Cluster mode enabled",
			"peer.count", len(cfg.Peers),
			"peers", fmt.Sprintf("%v", cfg.Peers),
		)
	} else {
		logger.Info("Single node mode - no peers configured")
	}

	// Initialize Node Service
	logger.Info("🔧 Initializing node service")
	svc := node.New(
		cfg.NodeUrl,
		cfg.VirtualNodes,
		cfg.Peers,
	)

	// Initialize HTTP Server
	logger.Info("🌐 Initializing HTTP server")
	srv := server.New(svc, cfg.ServerPort)

	// Start Server in a goroutine
	serverStarted := make(chan bool, 1)
	go func() {
		logger.Info("✅ LimeDB Node ready and listening",
			"port", cfg.ServerPort,
			"health.endpoint", fmt.Sprintf("http://localhost:%d/api/v1/health", cfg.ServerPort),
			"api.endpoint", fmt.Sprintf("http://localhost:%d/api/v1/", cfg.ServerPort),
		)
		serverStarted <- true

		if err := srv.Start(); err != nil {
			logger.Error("❌ Server error", "error", err)
		}
	}()

	// Wait for server to start
	<-serverStarted

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 Shutdown signal received, gracefully stopping server")
	if err := srv.Shutdown(); err != nil {
		logger.Warn("⚠️  Server forced to shutdown", "error", err)
	} else {
		logger.Info("✅ Server shutdown completed successfully")
	}

	logger.Info("👋 LimeDB Node exited")
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
