package main

import (
	"chord_dns/chord"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"time"
	"bufio"
    "os"
    "strings"
)


func runTestSuite(nodes []ChordNode, config *TestConfig) {
    // Check if only interactive test is requested
    if config.TestInteractive && !config.RunAll && !config.TestPing && !config.TestJoin && 
        !config.TestStabilize && !config.TestFingers && !config.TestOperations && !config.TestDHT {
        printSeparator("Interactive DNS Resolution Test")
        interactiveDNSTest(nodes)
        return
    }

    // If no flags are set (excluding interactive) or -all is used, run all tests
    if config.RunAll || (!config.TestPing && !config.TestJoin && !config.TestStabilize && 
        !config.TestFingers && !config.TestOperations && !config.TestDHT && !config.TestInteractive) {
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

	if config.TestReplication {
        printSeparator("Testing Replication")
        printReplicationStatus(nodes)
    }

	if config.TestSuccessors {
		printSeparator("Testing Successor Lists")
		testSuccessorLists(nodes)
	}
}

// Update runAllTests to optionally include interactive test
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

	printSeparator("Testing Successor Lists")
	testSuccessorLists(nodes)
	
    printSeparator("Testing Put and Get Operations")
    testPutAndGet(nodes)

    printSeparator("Printing DHTs for Each Node")
    testPrintDHTs(nodes)

	printSeparator("Testing Replication")
	printReplicationStatus(nodes)

	printSeparator("Testing Key Transfer on Late Join")
    testKeyTransferOnLateJoin(nodes)
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
	for iteration := 0; iteration < 1; iteration++ {
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
	for iteration := 0; iteration < 1; iteration++ {
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
        err := randomNode.node.ReplicatedPut(hashedKey, []byte(data.ip))
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
        for key, metadata := range node.node.DHT {
            fmt.Printf("  Key: %s, Value: %s (Primary: %v)\n", 
                key, string(metadata.Value), metadata.IsPrimary)
            if !metadata.IsPrimary {
                fmt.Printf("    Primary Node: %s\n", metadata.PrimaryNode.ID)
            }
        }
    }
}

func interactiveDNSTest(nodes []ChordNode) {
    fmt.Println("\n=== Interactive DNS Resolution Test ===")
    fmt.Println("Commands:")
    fmt.Println("1. put <domain> <ip>  - Store a new DNS record")
    fmt.Println("2. get <domain>       - Lookup a domain's IP")
    fmt.Println("3. exit              - Exit the interactive test")

    reader := bufio.NewReader(os.Stdin)

    for {
        fmt.Print("\nEnter command > ")
        input, err := reader.ReadString('\n')
        if err != nil {
            fmt.Printf("Error reading input: %v\n", err)
            continue
        }

        // Trim whitespace and split the input
        parts := strings.Fields(strings.TrimSpace(input))
        if len(parts) == 0 {
            continue
        }

        command := strings.ToLower(parts[0])

        switch command {
        case "put":
            if len(parts) != 3 {
                fmt.Println("Usage: put <domain> <ip>")
                continue
            }
            domain := parts[1]
            ip := parts[2]

            // Hash the domain name
            hashedKey := chord.HashKey(domain).String()
            
            // Pick a random node to initiate the put operation
            randomNode := nodes[rand.Intn(len(nodes))]
            
            // Store the DNS record
            err := randomNode.node.ReplicatedPut(hashedKey, []byte(ip))
            if err != nil {
                fmt.Printf("Failed to store DNS record for '%s': %v\n", domain, err)
                continue
            }

            // Find which node is responsible for this key
            keyBigInt := new(big.Int)
            keyBigInt.SetString(hashedKey, 10)
            responsibleNode := randomNode.node.FindResponsibleNode(keyBigInt)
            
            fmt.Printf("Successfully stored DNS record:\n")
            fmt.Printf("Domain: %s\n", domain)
            fmt.Printf("IP: %s\n", ip)
            fmt.Printf("Hash: %s\n", hashedKey)
            fmt.Printf("Stored on Node: %s\n", responsibleNode.ID)

        case "get":
            if len(parts) != 2 {
                fmt.Println("Usage: get <domain>")
                continue
            }
            domain := parts[1]

            // Hash the domain name
            hashedKey := chord.HashKey(domain).String()
            
            // Try retrieving from a random node
            var retrieved bool
            for _, node := range nodes {
                value, err := node.node.Get(hashedKey)
                if err != nil {
                    continue // Try next node if error occurs
                }
                
                if string(value) != "" {
                    fmt.Printf("DNS Resolution Result:\n")
                    fmt.Printf("Domain: %s\n", domain)
                    fmt.Printf("IP: %s\n", string(value))
                    fmt.Printf("Hash: %s\n", hashedKey)
                    fmt.Printf("Retrieved through Node: %s\n", node.node.ID)
                    retrieved = true
                    break
                }
            }

            if !retrieved {
                fmt.Printf("Failed to resolve domain '%s'. It might not exist in the DHT.\n", domain)
            }

        case "exit":
            fmt.Println("Exiting interactive DNS test...")
            return

        default:
            fmt.Println("Unknown command. Available commands: put, get, exit")
        }
    }
}

func printReplicationStatus(nodes []ChordNode) {
    fmt.Println("\n=== Replication Status Across Network ===")
    
    // Create a map to store all unique keys
    allKeys := make(map[string]bool)
    for _, node := range nodes {
        for key := range node.node.DHT {
            allKeys[key] = true
        }
    }
    
    // For each key, find all nodes that have it
    for key := range allKeys {
        fmt.Printf("\nKey: %s\n", key)
        
        // Find the primary node (node responsible for this key)
        keyBigInt := new(big.Int)
        keyBigInt.SetString(key, 10)
        primaryNode := nodes[0].node.FindResponsibleNode(keyBigInt)
        
        fmt.Printf("Primary Node:\n")
        primaryFound := false
        
        // Print primary node's information
        for i, node := range nodes {
            if node.node.ID.Cmp(primaryNode.ID) == 0 {
                if metadata, exists := node.node.DHT[key]; exists {
                    fmt.Printf("  - Node %d (ID: %s)\n", i+1, node.node.ID)
                    fmt.Printf("    Address: %s\n", node.node.Address)
                    fmt.Printf("    Value: %s\n", string(metadata.Value))
                    fmt.Printf("    Primary: %v\n", metadata.IsPrimary)
                    primaryFound = true
                }
                break
            }
        }
        
        if !primaryFound {
            fmt.Printf("  Warning: Primary node doesn't have the key!\n")
        }
        
        // Print replica nodes' information
        fmt.Printf("Replica Nodes:\n")
        replicaCount := 0
        
        for i, node := range nodes {
            // Skip if this is the primary node
            if node.node.ID.Cmp(primaryNode.ID) == 0 {
                continue
            }
            
            // Check if this node has the key
            if metadata, exists := node.node.DHT[key]; exists {
                fmt.Printf("  - Node %d (ID: %s)\n", i+1, node.node.ID)
                fmt.Printf("    Address: %s\n", node.node.Address)
                fmt.Printf("    Value: %s\n", string(metadata.Value))
                fmt.Printf("    Primary: %v\n", metadata.IsPrimary)
                if !metadata.IsPrimary && metadata.PrimaryNode != nil {
                    fmt.Printf("    Primary Node: %s\n", metadata.PrimaryNode.ID)
                }
                replicaCount++
            }
        }
        
        // Print replication summary
        fmt.Printf("\nReplication Summary:\n")
        fmt.Printf("  Total copies: %d (1 primary + %d replicas)\n", 
            replicaCount+1, replicaCount)
        
        if replicaCount+1 < chord.ReplicationFactor {
            fmt.Printf("  Warning: Current replication factor (%d) is below desired (%d)\n",
                replicaCount+1, chord.ReplicationFactor)
        } else {
            fmt.Printf("  Status: Achieved desired replication factor of %d\n",
                chord.ReplicationFactor)
        }
        
        fmt.Println(strings.Repeat("-", 50))
    }
}

func testSuccessorLists(nodes []ChordNode) {
    fmt.Println("Monitoring successor lists...")
    for iteration := 0; iteration < 1; iteration++ {	// Putting one iteration for now
        fmt.Printf("\n=== Iteration %d ===\n", iteration+1)
        time.Sleep(2 * time.Second)

        for i, node := range nodes {
            fmt.Printf("\nNode %d (ID: %s):\n", i+1, node.node.ID)
            fmt.Printf("Successor list (size: %d):\n", len(node.node.Successors))
            
            for j, successor := range node.node.Successors {
                if successor != nil {
                    fmt.Printf("  [%d] ID: %s, Address: %s\n", 
                        j, successor.ID, successor.Address)
                } else {
                    fmt.Printf("  [%d] nil\n", j)
                }
            }
        }
    }
}

func testKeyTransferOnLateJoin(nodes []ChordNode) {    
    // 1. Wait for initial stabilization
    fmt.Println("Waiting for ring to stabilize...")
    time.Sleep(5 * time.Second)
    
    // 2. Print current DHT state of all nodes
    fmt.Println("\nCurrent DHT state before new node:")
    testPrintDHTs(nodes)
    
    // 3. Create a new node with the next available port
    newPort := basePort + len(nodes)
    newAddress := fmt.Sprintf(":%d", newPort)
    
    newNode, err := chord.NewNode(newAddress, chord.NewHTTPNodeClient(newAddress))
    if err != nil {
        fmt.Printf("Failed to create new node: %v\n", err)
        return
    }
    
    // Start HTTP server for new node
    go http.ListenAndServe(newAddress, chord.NewHTTPNodeServer(newNode).SetupRoutes())
    time.Sleep(time.Second) // Wait for server to start
    
    // 4. Join the ring through the first node
    introducer := &chord.RemoteNode{
        ID:      nodes[0].node.ID,
        Address: nodes[0].node.Address,
        Client:  chord.NewHTTPNodeClient(nodes[0].node.Address),
    }
    
    fmt.Printf("\nNew node (ID: %s) joining the ring...\n", newNode.ID)
    err = newNode.Join(introducer)
    if err != nil {
        fmt.Printf("Failed to join ring: %v\n", err)
        return
    }
    
    // 5. Wait for stabilization and key transfer
    fmt.Println("Waiting for stabilization and key transfer...")
    time.Sleep(5 * time.Second)
    
    // 6. Print final DHT state of all nodes including new node
    fmt.Println("\nDHT state after new node joined:")
    newNodes := append(nodes, ChordNode{node: newNode})
    testPrintDHTs(newNodes)
    
    // 7. Print successor and predecessor info for verification
    fmt.Println("\nNew node's predecessor and successor:")
    if newNode.Predecessor != nil {
        fmt.Printf("Predecessor: %s\n", newNode.Predecessor.ID)
    } else {
        fmt.Println("Predecessor: nil")
    }
    if newNode.Successors[0] != nil {
        fmt.Printf("Successor: %s\n", newNode.Successors[0].ID)
    } else {
        fmt.Println("Successor: nil")
    }
}
