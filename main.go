package main

/*
options for terminal commands to run the project:
01 go run main.go test_suite.go base_case.go utils.go
02 go run .
*preferred 03a go build -o chord_dns
*preferred 03b ./chord_dns

flags:
-numNodes=20
-concurrentRequests=10
-case=base/case1/case2/case3/case4/test
-all
-ping
-join
-ops
-dht
-stabilize
-interactive
*/

import (
	"chord_dns/chord"
	"flag"
	"fmt"
	"log"
	"time"
)

const (
	basePort = 8001
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
	numNodes := flag.Int("numNodes", 10, "Number of Chord nodes to initialize (default: 10)")
	caseFlag := flag.String("case", "client", "Specify the case to run: client, base, case1, case2, case3, case4, or test")
	concurrentRequests := flag.Int("concurrentRequests", 10, "Number of concurrent lookup requests")
	flag.Parse()

	// Initialize Chord nodes
	nodes := initializeNodes(*numNodes)

	// Wait for servers to start
	time.Sleep(2 * time.Second)

	// Run the appropriate case based on the case flag
	switch *caseFlag {
	case "client":
		testNodeJoining(nodes)
		time.Sleep(5 * time.Second)
		runBaseCasePut(nodes)
		runClientInterface(nodes) // Run client interface
	case "base":
		fmt.Println("\nRunning the base case...")
		runBaseCase(nodes)
	case "case1":
		fmt.Println("\nRunning Case 1...")
		runCase1(nodes, *concurrentRequests)
	case "case2":
		fmt.Println("\nRunning Case 2...")
		runCase2(nodes)
	case "case3":
		fmt.Println("\nRunning Case 3...")
		runCase3(nodes)
	case "case4":
		fmt.Println("\nRunning Case 4...")
		runCase4(nodes)
	case "test":
		fmt.Println("\nRunning the test cases...")
		runTestSuite(nodes, config)
	default:
		log.Fatalf("\nUnknown case: %s. Use one of: base, case1, case2, case3, case4, test.", *caseFlag)
	}

	fmt.Println("\nServers running. Press Ctrl+C to exit.")
	select {}
}

func defineFlags() *TestConfig {
	config := &TestConfig{}

	// Define flags
	flag.BoolVar(&config.RunAll, "all", false, "Run All Tests")
	flag.BoolVar(&config.TestPing, "ping", false, "Test Node Connectivity")
	flag.BoolVar(&config.TestJoin, "join", false, "Test Node Joining")
	flag.BoolVar(&config.TestOperations, "ops", false, "Test Put and Get Operations")
	flag.BoolVar(&config.TestDHT, "dht", false, "Print DHTs for Each node")
	flag.BoolVar(&config.TestStabilize, "stabilize", false, "Test Stabilization")
	flag.BoolVar(&config.TestInteractive, "interactive", false, "Interactive DNS Resolution")

	return config
}

func initializeNodes(numNodes int) []ChordNode {
	nodes := make([]ChordNode, numNodes)
	for i := 0; i < numNodes; i++ {
		addr := fmt.Sprintf(":%d", basePort+i)
		node, server := createNode(addr)
		nodes[i] = ChordNode{node: node, server: server}
		startServer(server, addr)
	}
	return nodes
}
