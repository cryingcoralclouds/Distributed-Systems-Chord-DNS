package main

import (
	"chord_dns/chord"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	addr1 := ":8001"
	addr2 := ":8002"

	node1, server1 := createNode(addr1)
	node2, server2 := createNode(addr2)

	// Start servers
	startServer(server1, addr1)
	startServer(server2, addr2)

	// Wait for servers to start
	time.Sleep(time.Second)

	// Run tests

	// Basic node operations: Ping, Join, Stabilization
	printSeparator()
	testPing()

	printSeparator()
	testNodeJoining(node1, node2)

	printSeparator()
	testStabilization(node1, node2)

	printSeparator()
	testPut(node1, node2)

	// Core DHT functionalities: Put, Get

    fmt.Println("\nServers running. Press Ctrl+C to exit.")
    select {}
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

func testPing() {
	fmt.Println("Testing node connectivity...")
	urls := []string{
		"http://localhost:8001/ping",
		"http://localhost:8002/ping",
	}

	for _, url := range urls {
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("Error pinging %s: %v\n", url, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Printf("Successfully pinged %s\n", url)
		} else {
			fmt.Printf("Failed to ping %s: status code %d\n", url, resp.StatusCode)
		}
	}
}

func testNodeJoining(node1, node2 *chord.Node) {
	log.Println("Testing node joining...")

	introducer := &chord.RemoteNode{
		ID:      node1.ID,
		Address: node1.Address,
		Client:  chord.NewHTTPNodeClient(node1.Address),
	}

	log.Printf("Node 2 trying to join through introducer at %s", node1.Address)
	if err := node2.Join(introducer); err != nil {
		log.Fatalf("Node 2 failed to join: %v", err)
	}
	log.Println("Node 2 successfully joined through Node 1")
}

func testStabilization(node1, node2 *chord.Node) {
	log.Println("Testing stabilization...")
	fmt.Println("\nNode comparison:")
    fmt.Printf("Node 1 ID is %s Node 2 ID\n", 
        chord.CompareNodes(node1.ID, node2.ID))

    fmt.Println("\nWaiting for stabilization...")
	
    fmt.Println("Waiting for stabilization...")
    for i := 0; i < 5; i++ {
        time.Sleep(2 * time.Second)
        
        fmt.Printf("\nIteration %d:\n", i+1)
        
        // Node 1 details
        fmt.Printf("\nNode 1 (ID: %s, Address: %s):\n", node1.ID, node1.Address)
        if node1.Predecessor != nil {
            fmt.Printf("  Predecessor: %s (Address: %s)\n", 
                node1.Predecessor.ID, node1.Predecessor.Address)
        } else {
            fmt.Println("  Predecessor: nil")
        }
        fmt.Printf("  Successor: %s (Address: %s)\n", 
            node1.Successors[0].ID, node1.Successors[0].Address)

        // Node 2 details
        fmt.Printf("\nNode 2 (ID: %s, Address: %s):\n", node2.ID, node2.Address)
        if node2.Predecessor != nil {
            fmt.Printf("  Predecessor: %s (Address: %s)\n", 
                node2.Predecessor.ID, node2.Predecessor.Address)
        } else {
            fmt.Println("  Predecessor: nil")
        }
        fmt.Printf("  Successor: %s (Address: %s)\n", 
            node2.Successors[0].ID, node2.Successors[0].Address)
    }
}

func testPut(node1, node2 *chord.Node) {
	testKey := "test-key"
	testValue := []byte("test-value")

	// Try storing on node1
	err := node1.Put(testKey, testValue)
	if err != nil {
		log.Fatalf("Failed to put key-value: %v", err)
	}

	// Verify the key was stored in the correct node
	// Get the hash of the key
	keyHash := chord.HashKey(testKey)
	fmt.Printf("Key hash: %s\n", keyHash.String())
	fmt.Printf("Node1 ID: %s\n", node1.ID.String())
	fmt.Printf("Node2 ID: %s\n", node2.ID.String())

	// Wait a bit then check both nodes
	time.Sleep(time.Second)

	// Print where the key ended up
	if value, exists := node1.Keys[testKey]; exists {
		fmt.Printf("Key '%s' found in node1, value: %s\n", testKey, string(value))
	} else if value, exists := node2.Keys[testKey]; exists {
		fmt.Printf("Key '%s' found in node2, value: %s\n", testKey, string(value))
	} else {
		fmt.Println("Key not found in either node1 or node2")
	}
}
