package main

import (
	"chord_dns/chord"
	"fmt"
	"math/rand"
	"time"
)

func runBaseCase(nodes []ChordNode) {
	printSeparator("Initialising Base Case")
	testNodeJoining(nodes)

	// Allow time for stabilization and finger table setup
	time.Sleep(8 * time.Second)

	printSeparator("Putting Key-Value Pairs into DHTs")
	runBaseCasePut(nodes)

	printSeparator("Interactive DNS Resolution")
	interactiveDNS(nodes)
}

func runBaseCasePut(nodes []ChordNode) {
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
	}

	// Wait for potential replication/stabilization
	time.Sleep(2 * time.Second)
}
