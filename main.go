package main

import (
	"chord_dns/chord"
	"fmt"
	"log"
	"net/http"
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

func main() {
	// Create nodes
	nodes := make([]ChordNode, numNodes)
	for i := 0; i < numNodes; i++ {
		addr := fmt.Sprintf(":%d", basePort+i)
		node, server := createNode(addr)
		nodes[i] = ChordNode{node: node, server: server}
		startServer(server, addr)
	}

	// Wait for servers to start
	time.Sleep(2 * time.Second)

	// Run tests
	printSeparator("Testing Node Connectivity")
	testPing()

	printSeparator("Testing Node Joining")
	testNodeJoining(nodes)

	printSeparator("Testing Stabilization")
	testStabilization(nodes)

	printSeparator("Testing Put and Get Operations")
	testPutAndGet(nodes)

	fmt.Println("\nServers running. Press Ctrl+C to exit.")
	select {}
}

func printSeparator(title string) {
	fmt.Printf("\n=== %s ===\n\n", title)
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

func testPing() {
	for i := 0; i < numNodes; i++ {
		url := fmt.Sprintf("http://localhost:%d/ping", basePort+i)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("Error pinging %s: %v\n", url, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Printf("Successfully pinged node %d at %s\n", i+1, url)
		} else {
			fmt.Printf("Failed to ping node %d at %s: status code %d\n", i+1, url, resp.StatusCode)
		}
	}
}

func testNodeJoining(nodes []ChordNode) {
	// First node is the introducer
	introducer := &chord.RemoteNode{
		ID:      nodes[0].node.ID,
		Address: nodes[0].node.Address,
		Client:  chord.NewHTTPNodeClient(nodes[0].node.Address),
	}

	// Join other nodes through the introducer
	for i := 1; i < numNodes; i++ {
		log.Printf("Node %d trying to join through introducer\n", i+1)
		if err := nodes[i].node.Join(introducer); err != nil {
			log.Fatalf("Node %d failed to join: %v", i+1, err)
		}
		log.Printf("Node %d successfully joined the network\n", i+1)
		time.Sleep(500 * time.Millisecond) // Brief pause between joins
	}
}

func testStabilization(nodes []ChordNode) {
	fmt.Println("Monitoring network stabilization...")
	
	for iteration := 0; iteration < 5; iteration++ {
		time.Sleep(2 * time.Second)
		fmt.Printf("\nIteration %d:\n", iteration+1)
		
		for i, node := range nodes {
			fmt.Printf("\nNode %d (ID: %s, Address: %s):\n", i+1, node.node.ID, node.node.Address)
			
			if node.node.Predecessor != nil {
				fmt.Printf("  Predecessor: %s (Address: %s)\n", 
					node.node.Predecessor.ID, node.node.Predecessor.Address)
			} else {
				fmt.Println("  Predecessor: nil")
			}
			
			fmt.Printf("  Successor: %s (Address: %s)\n", 
				node.node.Successors[0].ID, node.node.Successors[0].Address)
		}
	}
}

func testPutAndGet(nodes []ChordNode) {
	testKeys := []string{
		"test-key-1",
		"test-key-2",
		"test-key-3",
	}
	testValues := []string{
		"test-value-1",
		"test-value-2",
		"test-value-3",
	}

	// Test Put operations
	fmt.Println("Testing Put operations:")
	for i, key := range testKeys {
		// Choose random node to put from
		nodeIndex := i % numNodes
		err := nodes[nodeIndex].node.Put(key, []byte(testValues[i]))
		if err != nil {
			log.Printf("Failed to put key-value %d: %v\n", i+1, err)
			continue
		}
		fmt.Printf("Successfully stored key '%s' through node %d\n", key, nodeIndex+1)
		
		// Print key location info
		keyHash := chord.HashKey(key)
		fmt.Printf("Key hash: %s\n", keyHash.String())
	}

	time.Sleep(2 * time.Second) // Wait for stabilization

	// Test Get operations
	fmt.Println("\nTesting Get operations:")
	for _, key := range testKeys {
		// Try getting through different nodes
		for i := 0; i < 3; i++ {
			nodeIndex := (i * 3) % numNodes // Test with different nodes
			value, err := nodes[nodeIndex].node.Get(key)
			if err != nil {
				fmt.Printf("Error getting '%s' through node %d: %v\n", 
					key, nodeIndex+1, err)
				continue
			}
			fmt.Printf("Successfully retrieved '%s' = '%s' through node %d\n", 
				key, string(value), nodeIndex+1)
			break
		}
	}

	// Test non-existent key
	fmt.Println("\nTesting Get with non-existent key:")
	_, err := nodes[0].node.Get("non-existent-key")
	if err != nil {
		fmt.Printf("Expected error getting non-existent key: %v\n", err)
	}
}