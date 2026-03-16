package main

import (
	"context"
	"fmt"
	"limedb/internal/config"
	"limedb/internal/gossiper"
	"limedb/internal/logger"
	"limedb/internal/messenger"
	"limedb/internal/node"
	"limedb/internal/server"
	"limedb/internal/store"
	"limedb/internal/store/lsm"
	"limedb/internal/telemetry"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/valyala/fasthttp"
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

	// Initialize Store
	lsmDir := filepath.Join(cfg.DataDir, cfg.NodeUrl+"_lsm")
	logger.Info("💾 Initializing LSM store", "dir", lsmDir)

	var backend store.Backend
	lsmStore, err := lsm.NewStore(lsm.Config{Dir: lsmDir})
	if err != nil {
		logger.Info("⚠️  LSM store failed, falling back to memory store", "error", err.Error())
		backend = store.NewMemory()
	} else {
		logger.Info("✅ LSM store ready", "dir", lsmDir, "keys", lsmStore.Count())
		backend = lsmStore
	}

	// Initialize Node Service
	logger.Info("🔧 Initializing node service")
	svc := node.NewService(
		cfg.NodeUrl,
		cfg.VirtualNodes,
		cfg.Peers,
		backend,
	)

	// Initialize Gossiper (only if we have peers)
	logger.Info("🗣️  Initializing gossip protocol")
	httpClient := &fasthttp.Client{}
	sender := messenger.NewFasthttpMessengeSender(httpClient)
	msngr := messenger.NewMessenger(sender)
	g := gossiper.NewGossiper(cfg.NodeUrl, cfg.Peers, msngr, svc)
	g.StartGossiping()

	// Initialize HTTP Server
	logger.Info("🌐 Initializing HTTP server")
	srv := server.NewServer(svc, g, cfg.ServerPort)

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
