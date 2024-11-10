package main

import (
	"chord_dns/chord"
	"fmt"
	"log"
	"net/http"
	"time"
)


func runTestSuite(nodes []ChordNode, config *TestConfig) {
	// If no flags are set or -all is used, run all tests
	if config.RunAll || (!config.TestPing && !config.TestJoin && !config.TestStabilize && 
		!config.TestFingers && !config.TestOperations && !config.TestDHT) {
		runAllTests(nodes)
		return
	}

	// Run individual tests based on flags
	if config.TestPing {
		printSeparator("Testing Node Connectivity")
		testPing(nodes)
	}

	if config.TestJoin {
		printSeparator("Testing Node Joining")
		testNodeJoining(nodes)
		// Allow time for initial setup
		time.Sleep(2 * time.Second)
	}

	if config.TestStabilize {
		printSeparator("Testing Stabilization")
		testStabilization(nodes)
	}

	if config.TestFingers {
		printSeparator("Testing Finger Tables")
		testFingerTables(nodes)
	}

	if config.TestOperations {
		printSeparator("Testing Put and Get Operations")
		testPutAndGet(nodes)
	}

	if config.TestDHT {
		printSeparator("Printing DHTs for Each Node")
		testPrintDHTs(nodes)
	}
}

func runAllTests(nodes []ChordNode) {
	printSeparator("Testing Node Connectivity")
	testPing(nodes)

	printSeparator("Testing Node Joining")
	testNodeJoining(nodes)

	// Allow time for stabilization and finger table setup
	time.Sleep(5 * time.Second)

	printSeparator("Testing Stabilization")
	testStabilization(nodes)

	printSeparator("Testing Finger Tables")
	testFingerTables(nodes)

	printSeparator("Testing Put and Get Operations")
	testPutAndGet(nodes)

	printSeparator("Printing DHTs for Each Node")
	testPrintDHTs(nodes)
}

func testPing(nodes []ChordNode) {
	for i := 0; i < len(nodes); i++ {
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
	log.Println("Node 1 creating new ring")
	if err := nodes[0].node.Join(nil); err != nil {
		log.Fatalf("Node 1 failed to create ring: %v", err)
	}
	log.Println("Node 1 successfully created new ring")
	time.Sleep(500 * time.Millisecond)

	introducer := &chord.RemoteNode{
		ID:      nodes[0].node.ID,
		Address: nodes[0].node.Address,
		Client:  chord.NewHTTPNodeClient(nodes[0].node.Address),
	}

	for i := 1; i < len(nodes); i++ {
		log.Printf("Node %d trying to join through introducer\n", i+1)
		if err := nodes[i].node.Join(introducer); err != nil {
			log.Fatalf("Node %d failed to join: %v", i+1, err)
		}
		log.Printf("Node %d successfully joined the network\n", i+1)
		time.Sleep(500 * time.Millisecond)
	}
}

func testStabilization(nodes []ChordNode) {
	fmt.Println("Monitoring network stabilization...")
	for iteration := 0; iteration < 5; iteration++ {
		time.Sleep(chord.StabilizeInterval)
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

func testFingerTables(nodes []ChordNode) {
	fmt.Println("Monitoring finger tables...")
	for iteration := 0; iteration < 3; iteration++ {
		fmt.Printf("\n=== Iteration %d ===\n", iteration+1)
		time.Sleep(2 * time.Second)

		for i, node := range nodes {
			fmt.Printf("\nNode %d (ID: %s):\n", i+1, node.node.ID)
			for j := 0; j < chord.M; j++ {
				entry := node.node.FingerTable[j]
				if entry != nil {
					fmt.Printf("  Finger[%d]: %s\n", j, entry.ID.String())
				} else {
					fmt.Printf("  Finger[%d]: nil\n", j)
				}
			}
		}
	}
}

func testPutAndGet(nodes []ChordNode) {
	testKeys := []string{"google.com", "wikipedia.org", "reddit.com"}
	testValues := []string{"172.217.20.206", "208.80.154.224", "151.101.193.140"}

	hashedKeys := make([]string, len(testKeys))
	for i, key := range testKeys {
		hashedKeys[i] = chord.HashKey(key).String()
	}

	fmt.Println("Testing Put operations:")
	for i, hashedKey := range hashedKeys {
		nodeIndex := i % len(nodes)
		err := nodes[nodeIndex].node.Put(hashedKey, []byte(testValues[i]))
		if err != nil {
			log.Printf("Failed to put key-value %d: %v\n", i+1, err)
			continue
		}
		fmt.Printf("Successfully stored hashed key '%s' through node %d\n", hashedKey, nodeIndex+1)
	}

	time.Sleep(2 * time.Second)

	fmt.Println("\nTesting Get operations:")
	for i, hashedKey := range hashedKeys {
		for j := 0; j < len(nodes); j++ {
			nodeIndex := (i + j) % len(nodes)
			value, err := nodes[nodeIndex].node.Get(hashedKey)
			if err != nil {
				fmt.Printf("Error getting hashed key '%s' through node %d: %v\n", hashedKey, nodeIndex+1, err)
				continue
			}
			fmt.Printf("Successfully retrieved hashed key '%s' = '%s' through node %d\n",
				hashedKey, string(value), nodeIndex+1)
			break
		}
	}
}

func testPrintDHTs(nodes []ChordNode) {
	fmt.Println("Distributed Hash Tables (DHTs) for each node:")
	for i, node := range nodes {
		fmt.Printf("\nNode %d (ID: %s):\n", i+1, node.node.ID.String())
		if len(node.node.DHT) == 0 {
			fmt.Println("  DHT is empty.")
			continue
		}
		for key, value := range node.node.DHT {
			fmt.Printf("  Key: %s, Value: %s\n", key, string(value))
		}
	}
}