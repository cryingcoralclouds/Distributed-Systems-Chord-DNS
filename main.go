package main

/*
options for terminal commands to run the project:
01 go run main.go test_suite.go utils.go
02 go run .
>>>03a go build -o chord_dns
>>>03b ./chord_dns
*/

import (
	"chord_dns/chord"
	"flag"
	"fmt"
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
	RunAll          bool
	TestPing        bool
	TestJoin        bool
	TestOperations  bool
	TestDHT         bool
	TestStabilize   bool
	TestInteractive bool
}

func main() {
	// Define and parse flags
	config := defineFlags()
	flag.Parse()

	// Initialize Chord nodes
	nodes := initializeNodes()

	// Wait for servers to start
	time.Sleep(2 * time.Second)

	// Run test suite with flags
	runTestSuite(nodes, config)

	fmt.Println("\nServers running. Press Ctrl+C to exit.")
	select {}
}

func defineFlags() *TestConfig {
	config := &TestConfig{}

	// Define flags
	flag.BoolVar(&config.RunAll, "all", false, "Run All Tests")
	flag.BoolVar(&config.TestPing, "alive", false, "Test Node Connectivity")
	flag.BoolVar(&config.TestJoin, "join", false, "Test Node Joining")
	flag.BoolVar(&config.TestOperations, "ops", false, "Test Put and Get Operations")
	flag.BoolVar(&config.TestDHT, "dht", false, "Print DHTs for Each node")
	flag.BoolVar(&config.TestStabilize, "stabilize", false, "Test Stabilization")
	flag.BoolVar(&config.TestInteractive, "interactive", false, "Interactive DNS Resolution")

	return config
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
