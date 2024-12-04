package main

import (
	"chord_dns/chord"
	"chord_dns/dns"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
)

const (
	basePort = 8001
	numNodes = 10
)

type ChordNode struct {
	node   *chord.Node
	server *chord.HTTPNodeServer
}

// TestConfig holds the flag values for different tests
type TestConfig struct {
	RunAll         bool
	TestPing       bool
	TestJoin       bool
	TestStabilize  bool
	TestFingers    bool
	TestOperations bool
	TestDHT        bool
	TestInteractive bool
	TestReplication bool
	TestSuccessors bool
}

func main() {
    // Get configuration from environment variables set by Docker
    nodeID, err := strconv.Atoi(os.Getenv("NODE_ID"))
    if err != nil {
        log.Fatalf("Invalid NODE_ID: %v", err)
    }

    port := os.Getenv("PORT")
    if port == "" {
        port = "8001"
    }

    isBootstrap := os.Getenv("BOOTSTRAP") == "true"
    bootstrapAddr := os.Getenv("BOOTSTRAP_ADDR")

    // Initialize single node
    addr := ":" + port
    node, server := createNode(addr)

    // If this is not the bootstrap node, join the network
    if !isBootstrap && bootstrapAddr != "" {
        // Create a RemoteNode for the bootstrap node with proper ID initialization
        introducer := &chord.RemoteNode{
            ID:      chord.HashKey(bootstrapAddr), // Initialize ID by hashing the address
            Address: bootstrapAddr,
            Client:  chord.NewHTTPNodeClient(bootstrapAddr),
        }

        log.Printf("Attempting to join network through introducer at %s", bootstrapAddr)
        if err := node.Join(introducer); err != nil {
            log.Printf("Warning: Failed to join network: %v", err)
        } else {
            log.Printf("Successfully joined network through introducer")
        }
    }

    // Start the server
    log.Printf("Starting node %d on %s (Bootstrap: %v)", nodeID, addr, isBootstrap)
    if err := server.Start(addr); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}

func defineFlags() *TestConfig {
	config := &TestConfig{}
	
	// Define flags
	flag.BoolVar(&config.RunAll, "all", false, "Run all tests")
	flag.BoolVar(&config.TestPing, "ping", false, "Test node connectivity")
	flag.BoolVar(&config.TestJoin, "join", false, "Test node joining")
	flag.BoolVar(&config.TestStabilize, "stabilize", false, "Test network stabilization")
	flag.BoolVar(&config.TestFingers, "fingers", false, "Test finger tables")
	flag.BoolVar(&config.TestOperations, "ops", false, "Test Put and Get operations")
	flag.BoolVar(&config.TestDHT, "dht", false, "Print DHTs for each node")
	flag.BoolVar(&config.TestInteractive, "interactive", false, "Run interactive DNS resolution test")
	flag.BoolVar(&config.TestReplication, "replication", false, "Test replication")
	
	return config
}

func processDNS(inputFile, outputFile string) {
	err := dns.ProcessDNSData(inputFile, outputFile)
	if err != nil {
		log.Fatalf("Failed to process DNS data: %v", err)
	}
	fmt.Println("Hashed DNS data written to", outputFile)
}

func initializeNodes() []ChordNode {
	nodes := make([]ChordNode, numNodes)
	for i := 0; i < numNodes; i++ {
		addr := fmt.Sprintf(":%d", basePort+i)
		node, server := createNode(addr)
		nodes[i] = ChordNode{node: node, server: server}
		startServer(server, addr)
	}
	return nodes
}