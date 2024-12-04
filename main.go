package main

import (
	"chord_dns/chord"
	"chord_dns/dns"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
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
    // Get configuration from environment variables
    nodeID := os.Getenv("NODE_ID")
    port := os.Getenv("PORT")
    if port == "" {
        port = "8001"
    }

    // Construct proper node address using container name
    nodeName := fmt.Sprintf("node%s", nodeID)
    nodeAddr := fmt.Sprintf("%s:%s", nodeName, port)

    isBootstrap := os.Getenv("BOOTSTRAP") == "true"
    bootstrapAddr := os.Getenv("BOOTSTRAP_ADDR")

    // Initialize node with full address
    addr := nodeAddr
    node, server := createNode(addr)

    // If this is not the bootstrap node, join the network
    if !isBootstrap && bootstrapAddr != "" {
        introducer := &chord.RemoteNode{
            ID:      chord.HashKey(bootstrapAddr),
            Address: bootstrapAddr,
            Client:  chord.NewHTTPNodeClient(bootstrapAddr),
        }

        log.Printf("Attempting to join network through introducer at %s", bootstrapAddr)
        // Add retry logic for joining
        maxRetries := 5
        for i := 0; i < maxRetries; i++ {
            if err := node.Join(introducer); err != nil {
                log.Printf("Join attempt %d failed: %v", i+1, err)
                time.Sleep(2 * time.Second)
                continue
            }
            log.Printf("Successfully joined network")
            break
        }
    }

    // Start the server with the local port for binding
    localAddr := fmt.Sprintf(":%s", port)
    log.Printf("Starting node %s on %s (Bootstrap: %v)", nodeID, addr, isBootstrap)
    if err := server.Start(localAddr); err != nil {
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