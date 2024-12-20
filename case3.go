package main

import (
	"chord_dns/chord"
	"fmt"
	"math/rand"
	"time"
)

func runCase3(nodes []ChordNode) {
	printSeparator("Case 2: Simulation of Permanent Faults")

	// Step [1]: Establish the Chord network
	fmt.Println("\nEstablishing initial Chord network...")
	testNodeJoining(nodes)

	// Wait for network stabilization
	fmt.Println("\nWaiting for network to stabilize...")
	time.Sleep(5 * time.Second)

	// Step [3]: Add test data to the network
	fmt.Println("\nAdding test DNS records to the network...")
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

	// Store test data through random nodes
	for _, data := range testData {
		hashedKey := chord.HashKey(data.domain).String()
		randomNode := nodes[rand.Intn(len(nodes))]
		err := randomNode.node.ReplicatedPut(hashedKey, []byte(data.ip))
		if err != nil {
			fmt.Printf("Failed to store domain '%s': %v\n", data.domain, err)
			continue
		}
		fmt.Printf("Stored domain '%s' (Hash: %s)\n", data.domain, hashedKey)
	}

	// Step [4]: Print the initial DHT state
	fmt.Println("\nInitial DHT state:")
	testPrintDHTs(nodes)

	// Step [5]: Simulate node departure (permanent fault)
	departingNodeIdx := rand.Intn(len(nodes)-1) + 1 // Pick a random node other than the first
	departingNode := nodes[departingNodeIdx]

	fmt.Printf("\nSimulating permanent fault of Node %d (ID: %s)...\n",
		departingNodeIdx+1, departingNode.node.ID)
	departingNode.node.SimulateFault()

	// Step [6]: Trigger stabilization and print new DHT state
	fmt.Println("\nTriggering stabilization and checking DHT state...")
	// Force a stabilization cycle
	for _, node := range nodes {
		if node.node.IsAlive {
			node.node.StartStabilize()
		}
	}
	// Allow a moment for stabilization to take effect
	time.Sleep(5 * time.Second)
	testPrintDHTs(nodes)

	// Step [7]: Validate ring structure
	fmt.Println("\nValidating ring connectivity after node fault:")
	validateRingConnectivity(nodes)

	// Step [8]: Verify data migration
	fmt.Println("\nVerifying data migration after node fault:")
	for _, data := range testData {
		validateKeyResponsibility(nodes, data.domain, data.ip, departingNodeIdx)
	}
}

func validateKeyResponsibility(nodes []ChordNode, key string, expectedValue string, departedIdx int) {
	hashedKey := chord.HashKey(key).String()
	for i, node := range nodes {
		if i == departedIdx || !node.node.IsAlive {
			continue // Skip departed or inactive nodes
		}
		value, err := node.node.Get(hashedKey)
		if err == nil && string(value) == expectedValue {
			fmt.Printf("Key '%s' (hash: %s) is correctly stored in Node %s\n", key, hashedKey, node.node.ID)
			return
		}
	}
	fmt.Printf("Error: Key '%s' (hash: %s) is missing or incorrect after node departure\n", key, hashedKey)
}

func validateRingConnectivity(nodes []ChordNode) {
	fmt.Println("\nValidating ring connectivity...")
	for _, node := range nodes {
		if !node.node.IsAlive {
			continue // Skip inactive nodes
		}
		if node.node.Predecessor == nil || node.node.Successors[0] == nil {
			fmt.Printf("Error: Node %s has incomplete ring connections\n", node.node.ID)
			continue
		}
		pred := node.node.Predecessor
		succ := node.node.Successors[0]
		fmt.Printf("Node %s: Predecessor: %s, Successor: %s\n", node.node.ID, pred.ID, succ.ID)
	}
}
