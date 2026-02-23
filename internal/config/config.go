package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the application configuration.
type Config struct {
	ServerPort   int
	NodeUrl      string
	Peers        []string
	VirtualNodes int
	OtelEndpoint string
}

// Load parses command line arguments and returns the configuration.
func Load() *Config {
	// Helper functions to get env var or fallback
	getEnv := func(key, fallback string) string {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		return fallback
	}

	getEnvInt := func(key string, fallback int) int {
		if valueStr, ok := os.LookupEnv(key); ok {
			if parsed, err := strconv.Atoi(valueStr); err == nil {
				return parsed
			}
		}
		return fallback
	}

	// Read from ENV or use defaults
	defaultPort := getEnvInt("SERVER_PORT", 8484)
	defaultNodeUrl := getEnv("NODE_URL", "")
	defaultPeers := getEnv("NODE_PEERS", "")
	defaultVirtualNodes := getEnvInt("VIRTUAL_NODES", 256)
	defaultOtel := getEnv("OTEL_ENDPOINT", "localhost:4317")

	serverPort := flag.Int("server.port", defaultPort, "Server Port")
	nodeUrl := flag.String("node.url", defaultNodeUrl, "This node's URL (REQUIRED, e.g., http://192.168.1.125:8484)")
	peersStr := flag.String("node.peers", defaultPeers, "Comma-separated list of peer URLs")
	virtualNodes := flag.Int("node.routing.virtual-nodes", defaultVirtualNodes, "Number of virtual nodes per physical node")
	otelEndpoint := flag.String("otel.endpoint", defaultOtel, "OTLP Collector Endpoint (e.g., localhost:4317)")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  ./limedb -node.url <this_node_url> [options]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Examples:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  Standalone mode:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    ./limedb -node.url http://192.168.1.100:8484\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  Cluster mode (requires at least one external peer):\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    ./limedb -node.url http://192.168.1.100:8484 -node.peers http://192.168.1.101:8484\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    ./limedb -node.url http://192.168.1.101:8484 -node.peers http://192.168.1.100:8484,http://192.168.1.102:8484\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Check for required parameters
	if *nodeUrl == "" {
		fmt.Printf("Error: -node.url is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Check for non-flag arguments which might indicate user error (e.g. "server.port 7002" without -)
	if flag.NArg() > 0 {
		fmt.Printf("Error: Unexpected arguments: %v\n", flag.Args())
		fmt.Println("Did you forget a dash? Example: -server.port 7002")
		flag.Usage()
		os.Exit(1)
	}

	peers := strings.Split(*peersStr, ",")
	// Trim spaces from peers and filter out empty strings
	validPeers := make([]string, 0)
	for _, p := range peers {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			validPeers = append(validPeers, trimmed)
		}
	}

	// Validate cluster configuration
	if len(validPeers) > 0 {
		// Filter out self-reference from peers
		externalPeers := make([]string, 0)
		for _, peer := range validPeers {
			if peer != *nodeUrl {
				externalPeers = append(externalPeers, peer)
			}
		}

		// Require at least one external peer for cluster mode
		if len(externalPeers) == 0 {
			fmt.Printf("Error: At least one external peer is required for cluster mode\n")
			fmt.Printf("Current node: %s\n", *nodeUrl)
			fmt.Printf("Provided peers: %v\n", validPeers)
			fmt.Printf("\nFor cluster deployment, provide at least one other node's URL:\n")
			fmt.Printf("  ./limedb -node.url http://192.168.1.100:8484 -node.peers http://192.168.1.101:8484\n\n")
			fmt.Printf("For standalone mode, omit the -node.peers parameter:\n")
			fmt.Printf("  ./limedb -node.url http://192.168.1.100:8484\n\n")
			os.Exit(1)
		}

		validPeers = externalPeers
	} else {
		// Standalone mode: no peers provided warn the user
		fmt.Printf("⚠️  No peers configured - running in standalone mode\n")
	}

	return &Config{
		ServerPort:   *serverPort,
		NodeUrl:      *nodeUrl,
		Peers:        validPeers,
		VirtualNodes: *virtualNodes,
		OtelEndpoint: *otelEndpoint,
	}
}
