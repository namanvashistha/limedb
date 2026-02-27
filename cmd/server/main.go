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
	"limedb/internal/telemetry"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

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
	// Derive node ID from URL (e.g. "http://node1:8484" → "node1")
	nodeID := cfg.NodeUrl
	if u, err := url.Parse(cfg.NodeUrl); err == nil {
		nodeID = strings.Split(u.Hostname(), ".")[0] // e.g. "node1"
	}
	storeFilePath := filepath.Join(cfg.DataDir, nodeID+".json")
	logger.Info("💾 Initializing filesystem store", "path", storeFilePath)

	var backend store.Backend
	fsStore, err := store.NewFileSystem(storeFilePath)
	if err != nil {
		logger.Info("⚠️  Filesystem store failed, falling back to memory store", "error", err.Error())
		backend = store.NewMemory()
	} else {
		logger.Info("✅ Filesystem store ready", "path", storeFilePath, "keys", fsStore.Count())
		backend = fsStore
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
	g := gossiper.NewGossiper(cfg.NodeUrl, cfg.Peers, msngr)
	g.StartGossiping()

	// Initialize HTTP Server
	logger.Info("🌐 Initializing HTTP server")
	srv := server.NewServer(svc, g, cfg.ServerPort)

	// Start periodic ring synchronization with gossip
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Get active peers from gossip
			gossipMetrics := g.GetGossipMetrics()
			if peerDetails, ok := gossipMetrics["peer_details"].([]interface{}); ok {
				activePeers := make([]string, 0)
				for _, p := range peerDetails {
					if peerMap, ok := p.(map[string]interface{}); ok {
						if url, ok := peerMap["url"].(string); ok {
							// Add active and stale peers to the ring (exclude only dead peers)
							if status, ok := peerMap["status"].(string); ok && (status == "active" || status == "stale") {
								activePeers = append(activePeers, url)
							}
						}
					}
				}
				// Sync the ring with active gossip peers
				svc.SyncWithGossip(activePeers)
			}
		}
	}()

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
