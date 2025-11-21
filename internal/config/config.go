package config

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// Config holds the application configuration.
type Config struct {
	NodeID       int
	ServerPort   int
	Peers        []string
	VirtualNodes int
}

// Load parses command line arguments and returns the configuration.
func Load() *Config {
	nodeID := flag.Int("node.id", 1, "Node ID")
	serverPort := flag.Int("server.port", 8484, "Server Port")
	peersStr := flag.String("node.peers", "http://localhost:8484,http://localhost:7002", "Comma-separated list of peer URLs")
	virtualNodes := flag.Int("node.routing.virtual-nodes", 2, "Number of virtual nodes per physical node")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  ./limedb -node.id <id> -server.port <port> [options]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Check for non-flag arguments which might indicate user error (e.g. "server.port 7002" without -)
	if flag.NArg() > 0 {
		fmt.Printf("Error: Unexpected arguments: %v\n", flag.Args())
		fmt.Println("Did you forget a dash? Example: -server.port 7002")
		flag.Usage()
		os.Exit(1)
	}

	peers := strings.Split(*peersStr, ",")
	// Trim spaces from peers
	for i, p := range peers {
		peers[i] = strings.TrimSpace(p)
	}

	return &Config{
		NodeID:       *nodeID,
		ServerPort:   *serverPort,
		Peers:        peers,
		VirtualNodes: *virtualNodes,
	}
}
