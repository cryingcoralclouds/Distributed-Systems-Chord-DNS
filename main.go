package main

import (
	"chord_dns/chord"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type TestFunction func([]*chord.Node)

func main() {
	// Define flags
	numNodesFlag := flag.Int("nodes", 10, "Number of nodes in the network")
	basePortFlag := flag.Int("baseport", 8001, "Base port number")
	testFlag := flag.String("test", "", "Test to run (ping, join, stabilize, finger, putget, all)")
	keepAliveFlag := flag.Bool("keepalive", false, "Keep servers running after tests complete")
	flag.Parse()

	if *testFlag == "" {
		fmt.Println("Please specify a test to run using -test flag")
		fmt.Println("Available tests: ping, join, stabilize, finger, putget, all")
		return
	}

	// Create test function mapping
	testFunctions := map[string]TestFunction{
		"ping":      testPing,
		"join":      testNodeJoining,
		"stabilize": testStabilization,
		"finger":    testFingerTable,
		"putget":    testPutAndGet,
	}

	// Initialize nodes
	nodes := make([]*chord.Node, *numNodesFlag)
	servers := make([]*chord.HTTPNodeServer, *numNodesFlag)

	// Create and start all nodes
	for i := 0; i < *numNodesFlag; i++ {
		addr := ":" + strconv.Itoa(*basePortFlag+i)
		nodes[i], servers[i] = createNode(addr)
		startServer(servers[i], addr)
	}

	// Wait for servers to start
	time.Sleep(time.Second)

	// Run selected test(s)
	selectedTest := strings.ToLower(*testFlag)
	if selectedTest == "all" {
		for name, testFunc := range testFunctions {
			printSeparator()
			fmt.Printf("Running %s test...\n", name)
			testFunc(nodes)
		}
	} else if testFunc, exists := testFunctions[selectedTest]; exists {
		printSeparator()
		fmt.Printf("Running %s test...\n", selectedTest)
		testFunc(nodes)
	} else {
		fmt.Printf("Unknown test: %s\n", selectedTest)
		fmt.Println("Available tests: ping, join, stabilize, finger, putget, all")
		return
	}

	if *keepAliveFlag {
		fmt.Println("\nServers running. Press Ctrl+C to exit.")
		select {}
	} else {
		fmt.Println("\nTests completed. Shutting down servers.")
		// Cleanup: Stop all nodes
		for _, node := range nodes {
			if err := node.Leave(); err != nil {
				log.Printf("Error stopping node: %v", err)
			}
		}
		os.Exit(0)
	}
}

func printSeparator() {
	fmt.Println("\n--------------------------\n")
}

func createNode(addr string) (*chord.Node, *chord.HTTPNodeServer) {
	client := chord.NewHTTPNodeClient(addr)
	node, err := chord.NewNode(addr, client)
	if err != nil {
		log.Fatalf("Failed to create node at %s: %v", addr, err)
	}

	server := chord.NewHTTPNodeServer(node)
	return node, server
}

func startServer(server *chord.HTTPNodeServer, addr string) {
	go func() {
		if err := server.Start(addr); err != nil {
			log.Printf("Server at %s failed: %v", addr, err)
		}
	}()
}

func testPing(nodes []*chord.Node) {
	fmt.Println("Testing node connectivity...")

	for i, node := range nodes {
		url := fmt.Sprintf("http://localhost%s/ping", node.Address)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("Error pinging node %d at %s: %v\n", i, url, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Printf("Successfully pinged node %d at %s\n", i, url)
		} else {
			fmt.Printf("Failed to ping node %d at %s: status code %d\n", i, url, resp.StatusCode)
		}
	}
}

func testNodeJoining(nodes []*chord.Node) {
	log.Println("Testing node joining...")

	// Use the first node as the introducer
	introducer := &chord.RemoteNode{
		ID:      nodes[0].ID,
		Address: nodes[0].Address,
		Client:  chord.NewHTTPNodeClient(nodes[0].Address),
	}

	// Have all other nodes join through the first node
	for i := 1; i < len(nodes); i++ {
		log.Printf("Node %d trying to join through introducer at %s", i, nodes[0].Address)
		if err := nodes[i].Join(introducer); err != nil {
			log.Fatalf("Node %d failed to join: %v", i, err)
		}
		log.Printf("Node %d successfully joined through Node 0", i)
		// Small delay between joins to allow for stabilization
		time.Sleep(500 * time.Millisecond)
	}
}

func testStabilization(nodes []*chord.Node) {
	log.Println("Testing stabilization...")

	fmt.Println("\nNode comparison matrix:")
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			fmt.Printf("Node %d ID is %s Node %d ID\n",
				i, chord.CompareNodes(nodes[i].ID, nodes[j].ID), j)
		}
	}

	fmt.Println("\nWaiting for stabilization...")
	for iteration := 0; iteration < 5; iteration++ {
		time.Sleep(2 * time.Second)
		fmt.Printf("\nIteration %d:\n", iteration+1)

		for i, node := range nodes {
			fmt.Printf("\nNode %d (ID: %s, Address: %s):\n", i, node.ID, node.Address)
			if node.Predecessor != nil {
				fmt.Printf("  Predecessor: %s (Address: %s)\n",
					node.Predecessor.ID, node.Predecessor.Address)
			} else {
				fmt.Println("  Predecessor: nil")
			}
			fmt.Printf("  Successor: %s (Address: %s)\n",
				node.Successors[0].ID, node.Successors[0].Address)
		}
	}
}

func testPutAndGet(nodes []*chord.Node) {
	// Test multiple key-value pairs
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
	}

	fmt.Println("Testing Put operations...")
	for key, value := range testData {
		// Choose a random node to put the data
		nodeIndex := time.Now().UnixNano() % int64(len(nodes))
		node := nodes[nodeIndex]

		err := node.Put(key, []byte(value))
		if err != nil {
			log.Printf("Failed to put key '%s': %v\n", key, err)
			continue
		}

		keyHash := chord.HashKey(key)
		fmt.Printf("\nKey: %s\nHash: %s\nPut through Node %d (ID: %s)\n",
			key, keyHash.String(), nodeIndex, node.ID.String())
	}

	// Wait for data to propagate
	time.Sleep(time.Second)

	fmt.Println("\nVerifying data storage location...")
	for key := range testData {
		found := false
		for i, node := range nodes {
			if value, exists := node.DHT[key]; exists {
				fmt.Printf("Key '%s' found in Node %d, value: %s\n",
					key, i, string(value))
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("Key '%s' not found in any node\n", key)
		}
	}

	printSeparator()
	fmt.Println("Testing Get operations...")

	// Try getting each key through different nodes
	for key, expectedValue := range testData {
		for i, node := range nodes {
			value, err := node.Get(key)
			if err != nil {
				fmt.Printf("Error getting key '%s' through Node %d: %v\n",
					key, i, err)
				continue
			}
			if string(value) == expectedValue {
				fmt.Printf("Successfully retrieved key '%s' through Node %d: %s\n",
					key, i, string(value))
			} else {
				fmt.Printf("Value mismatch for key '%s' through Node %d: expected '%s', got '%s'\n",
					key, i, expectedValue, string(value))
			}
			break // Test one successful retrieval per key
		}
	}

	// Test getting a non-existent key
	fmt.Println("\nTesting Get with non-existent key:")
	if _, err := nodes[0].Get("non-existent-key"); err != nil {
		fmt.Printf("Expected error getting non-existent key: %v\n", err)
	}
}

func testFingerTable(nodes []*chord.Node) {
	fmt.Println("Testing finger table maintenance...")

	// Wait for finger tables to be populated
	time.Sleep(5 * time.Second)

	// Print finger table entries for all nodes
	for _, node := range nodes {
		fmt.Printf("\nNode %d Finger Table:\n", node.ID)
		for j, finger := range node.FingerTable {
			if finger != nil {
				fmt.Printf("Entry %d -> Node %s (Address: %s)\n",
					j, finger.ID, finger.Address)
			} else {
				fmt.Printf("Entry %d -> nil\n", j)
			}
		}
	}
}
