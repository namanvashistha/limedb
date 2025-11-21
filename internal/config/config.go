package config

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// Config holds the application configuration.
type Config struct {
	ServerPort   int
	NodeUrl      string
	Peers        []string
	VirtualNodes int
}

// Load parses command line arguments and returns the configuration.
func Load() *Config {
	serverPort := flag.Int("server.port", 8484, "Server Port")
	nodeUrl := flag.String("node.url", "", "This node's URL (REQUIRED, e.g., http://192.168.1.125:8484)")
	peersStr := flag.String("node.peers", "", "Comma-separated list of peer URLs")
	virtualNodes := flag.Int("node.routing.virtual-nodes", 2, "Number of virtual nodes per physical node")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  ./limedb -server.port <port> -node.url <url> [options]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Example:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  ./limedb -server.port 8484 -node.url http://192.168.1.125:8484 -node.peers http://192.168.1.125:8484,http://192.168.1.126:8484\n\n")
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
	// Trim spaces from peers
	for i, p := range peers {
		peers[i] = strings.TrimSpace(p)
	}

	return &Config{
		ServerPort:   *serverPort,
		NodeUrl:      *nodeUrl,
		Peers:        peers,
		VirtualNodes: *virtualNodes,
	}
}
