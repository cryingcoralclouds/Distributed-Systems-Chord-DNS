package main

import (
	"chord_dns/chord"
	"fmt"
	"log"
	"math/big"
	"math/rand"
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
    testData := []struct {
        domain string
        ip     string
    }{
        {"google.com", "172.217.20.206"},
        {"wikipedia.org", "208.80.154.224"},
        {"reddit.com", "151.101.193.140"},
        {"facebook.com", "31.13.77.36"},
        {"youtube.com", "74.125.65.91"},
        {"amazon.com", "205.251.242.103"},
        {"twitter.com", "199.59.149.230"},
        {"linkedin.com", "108.174.10.10"},
        {"instagram.com", "157.240.241.174"},
		{"netflix.com", "52.6.137.65"},
        {"yahoo.com", "98.137.149.56"},
        {"microsoft.com", "40.76.4.15"},
        {"apple.com", "17.253.144.10"},
    }

    fmt.Println("\nTesting Put operations:")
    // Store each key-value pair through any node - the ring will handle routing
    for _, data := range testData {
        // Hash the domain name
        hashedKey := chord.HashKey(data.domain).String()
        
        // Pick a random node to initiate the put operation
        randomNode := nodes[rand.Intn(len(nodes))]
        
        // Store the key-value pair
        err := randomNode.node.Put(hashedKey, []byte(data.ip))
        if err != nil {
            fmt.Printf("   Failed to put domain '%s' (hash: %s): %v\n", 
                data.domain, hashedKey, err)
            continue
        }
        
        // Find which node is actually responsible for this key
        keyBigInt := new(big.Int)
        keyBigInt.SetString(hashedKey, 10)
        responsibleNode := randomNode.node.FindResponsibleNode(keyBigInt)
        
        fmt.Printf("   Successfully stored domain '%s'\n", data.domain)
        fmt.Printf("   Hash: %s\n", hashedKey)
        fmt.Printf("   Responsible Node: %s\n", responsibleNode.ID)
        fmt.Printf("   IP: %s\n\n", data.ip)
    }

    // Wait for potential replication/stabilization
    time.Sleep(2 * time.Second)

    fmt.Println("\nTesting Get operations:")
    // Try to retrieve each key-value pair from different nodes
    for _, data := range testData {
        hashedKey := chord.HashKey(data.domain).String()
        
        // Try retrieving from each node to verify routing works
        var retrieved bool
        for _, node := range nodes {
            value, err := node.node.Get(hashedKey)
            if err != nil {
                continue // Try next node
            }
            
            if string(value) == data.ip {
                fmt.Printf("   Successfully retrieved domain '%s'\n", data.domain)
                fmt.Printf("   Hash: %s\n", hashedKey)
                fmt.Printf("   Retrieved through Node: %s\n", node.node.ID)
                fmt.Printf("   IP: %s\n\n", string(value))
                retrieved = true
                break
            } else {
                fmt.Printf("   Value mismatch for domain '%s'\n", data.domain)
                fmt.Printf("   Expected: %s\n", data.ip)
                fmt.Printf("   Got: %s\n\n", string(value))
            }
        }
        
        if !retrieved {
            fmt.Printf("   Failed to retrieve domain '%s' from any node\n", data.domain)
            fmt.Printf("   Hash: %s\n\n", hashedKey)
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