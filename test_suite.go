package main

import (
	"bufio"
	"chord_dns/chord"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
)

func runTestSuite(nodes []ChordNode, config *TestConfig) {
	// Check if only interactive test flag is set
	if config.TestInteractive && !config.RunAll && !config.TestPing && !config.TestJoin && !config.TestOperations && !config.TestDHT &&
		!config.TestStabilize {
		printSeparator("Interactive DNS Resolution")
		testNodeJoining(nodes)
		interactiveDNS(nodes)
		return
	}

	// Check if no flags are set or -all flag is set
	// Run all tests
	if config.RunAll || (!config.TestPing && !config.TestJoin &&
		!config.TestOperations && !config.TestDHT && !config.TestStabilize && !config.TestInteractive) {
		runAllTests(nodes)
		return
	}

	// Run individual tests based on flags set
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

	if config.TestOperations {
		printSeparator("Testing Put and Get Operations")
		testPutAndGet(nodes)
	}

	if config.TestDHT {
		printSeparator("Printing DHTs for Each Node")
		testPrintDHTs(nodes)
	}

	if config.TestStabilize {
		printSeparator("Testing Stabilization")
		testStabilization(nodes)
	}
}

func runAllTests(nodes []ChordNode) {
	printSeparator("Testing Node Connectivity")
	testPing(nodes)

	printSeparator("Testing Node Joining")
	testNodeJoining(nodes)

	// Allow time for stabilization and finger table setup
	time.Sleep(5 * time.Second)

	printSeparator("Testing Put and Get Operations")
	testPutAndGet(nodes)

	printSeparator("Printing DHTs for Each Node")
	testPrintDHTs(nodes)

	printSeparator("Testing Stabilization")
	testStabilization(nodes)
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

func testPutAndGet(nodes []ChordNode) {
	testData := []struct {
		domain string
		ip     string
	}{
		{"google.com", "172.217.20.206"},
		{"googleapis.com", "142.250.74.234"},
		{"facebook.com", "163.70.128.35"},
		{"googletagmanager.com", "142.250.184.232"},
		{"youtube.com", "142.250.178.14"},
		{"instagram.com", "157.240.214.174"},
		{"twitter.com", "104.244.42.65"},
		{"amazon.com", "205.251.242.103"},
		{"wikipedia.org", "208.80.154.224"},
		{"openai.com", "104.18.33.45"},
		{"cloudflare.com", "104.21.77.216"},
		{"github.com", "140.82.113.4"},
		{"microsoft.com", "20.70.246.20"},
		{"apple.com", "17.253.144.10"},
		{"reddit.com", "151.101.193.140"},
		{"linkedin.com", "108.174.10.10"},
		{"netflix.com", "52.6.137.65"},
		{"yahoo.com", "98.137.149.56"},
	}

	fmt.Println("Testing Put Operations:")
	// Store each key-value pair through any node - the Chord ring will handle routing
	for _, data := range testData {
		// Hash the domain name
		hashedKey := chord.HashKey(data.domain).String()

		// Pick a random node to initiate the Put operation
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

	fmt.Println("Testing Get Operations:")
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

func testStabilization(nodes []ChordNode) {
	fmt.Println("Monitoring network stabilization...")

	for iteration := 0; iteration < 1; iteration++ { // 1 stabilization iteration
		time.Sleep(chord.StabilizeInterval)
		fmt.Printf("\nIteration %d:\n", iteration+1)

		for i, node := range nodes {
			fmt.Printf("\nNode %d (ID: %s, Address: %s):\n", i+1, node.node.ID, node.node.Address)

			// 1. Test predecessor stabilization
			fmt.Println("Testing predecessor stabilization...")
			if node.node.Predecessor != nil {
				fmt.Printf("Predecessor: ID: %s, Address: %s\n",
					node.node.Predecessor.ID, node.node.Predecessor.Address)
			} else {
				fmt.Println("Predecessor: nil")
			}

			// 2. Test successor stabilization
			fmt.Println("Testing successor stabilization...")
			successorCount := 0
			for j, successor := range node.node.Successors {
				if successor != nil {
					fmt.Printf("Successor[%d]: ID: %s, Address: %s\n",
						j, successor.ID, successor.Address)
					successorCount++
				} else {
					fmt.Printf("Successor[%d]: nil\n", j)
				}
			}
			if successorCount < chord.NumSuccessors {
				fmt.Printf("Warning: Only %d successors found (expected at least %d)\n",
					successorCount, chord.ReplicationFactor)
			} else {
				fmt.Printf("Successor list is complete.\n")
			}

			// 3. Test finger table stabilization
			fmt.Println("Testingfinger table stabilization...")
			validFingers := 0
			for j := 0; j < chord.M; j++ {
				entry := node.node.FingerTable[j]
				if entry != nil {
					fmt.Printf("Finger[%d]: ID: %s, Address: %s\n", j, entry.ID.String(), entry.Address)
					validFingers++
				} else {
					fmt.Printf("Finger[%d]: nil\n", j)
				}
			}
			if validFingers < chord.M {
				fmt.Printf("Warning: Only %d/%d finger table entries are valid.\n", validFingers, chord.M)
			} else {
				fmt.Printf("Finger table is complete.\n")
			}
		}

		// 4. Test replication stabilization
		fmt.Println("Testing replication stabilization...")
		printReplicationStatus(nodes)
	}
}

func interactiveDNS(nodes []ChordNode) {
	fmt.Println("=== Interactive DNS Resolution Test ===")
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

			// Pick a random node to initiate the Put operation
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
