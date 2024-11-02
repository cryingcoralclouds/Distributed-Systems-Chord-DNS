package main

import (
	"chord_dns/chord"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	/* ------------------- Start clients and servers on 2 nodes ----------------------- */
    // Create two nodes on different ports
    addr1 := ":8001"
    addr2 := ":8002"

    // Create first node
    client1 := chord.NewHTTPNodeClient(addr1)
    node1, err := chord.NewNode(addr1, client1)
    if err != nil {
        log.Fatalf("Failed to create node 1: %v", err)
    }

    // Create second node
    client2 := chord.NewHTTPNodeClient(addr2)
    node2, err := chord.NewNode(addr2, client2)
    if err != nil {
        log.Fatalf("Failed to create node 2: %v", err)
    }

    // Start servers
    server1 := chord.NewHTTPNodeServer(node1)
    server2 := chord.NewHTTPNodeServer(node2)

    go func() {
        if err := server1.Start(addr1); err != nil {
            log.Printf("Server 1 failed: %v", err)
        }
    }()

    go func() {
        if err := server2.Start(addr2); err != nil {
            log.Printf("Server 2 failed: %v", err)
        }
    }()

    // Wait for servers to start
    time.Sleep(time.Second)

	/* ------------------- Test: Ping ----------------------- */
    // Test ping endpoints
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

	/* ------------------- Test: Node Joining ----------------------- */
    // Test node joining
    log.Println("Testing node joining...")
    
    introducer := &chord.RemoteNode{
        ID:      node1.ID,
        Address: addr1,
        Client:  chord.NewHTTPNodeClient(addr1),
    }

    log.Printf("Node 2 trying to join through introducer at %s", addr1)
    if err := node2.Join(introducer); err != nil {
        log.Fatalf("Node 2 failed to join: %v", err)
    }
    log.Println("Node 2 successfully joined through Node 1")

	/* ------------------- Test: Node Joining ----------------------- */
    log.Println("Testing stabilization...")
    testStabilization(node1, node2)

    fmt.Println("\nServers running. Press Ctrl+C to exit.")

	/* ------------------- local HTTP for now: End ----------------------- */
    select {}
}

func testStabilization(node1, node2 *chord.Node) {
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
