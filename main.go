package main

import (
	"chord_dns/chord"
	"chord_dns/dns"
	"flag"
	"fmt"
	"log"
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
	// Define and parse flags
	config := defineFlags()
	flag.Parse()

	// Process DNS data
	inputFile := "dns/dns_data.json"
	outputFile := "dns/hashed_dns_data.json"
	processDNS(inputFile, outputFile)

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