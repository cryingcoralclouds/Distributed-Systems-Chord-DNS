package main

import (
	"chord_dns/chord"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

func runCase2(nodes []ChordNode) {
	printSeparator("Case 2: Intermittent Node Faults")

	printSeparator("Initialising Base Case")
	testNodeJoining(nodes)

	// Allow time for stabilization and finger table setup
	time.Sleep(5 * time.Second)

	printSeparator("Putting Key-Value Pairs into DHTs")
	runBaseCasePut(nodes)

	// Wait for stabilization and replication
	time.Sleep(5 * time.Second)

	// Simulate periodic lookup requests with intermittent node faults
	var wg sync.WaitGroup
	faultInterval := time.Second * 5
	lookupInterval := time.Second * 2
	stopCh := make(chan struct{})

	// Simulate periodic lookups
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stopCh:
				return
			default:
				simulateLookup(nodes)
				time.Sleep(lookupInterval)
			}
		}
	}()

	// Simulate intermittent node faults
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stopCh:
				return
			default:
				introduceFault(nodes)
				time.Sleep(faultInterval)
			}
		}
	}()

	// Let the simulation run for a specified duration
	duration := time.Minute * 1
	fmt.Printf("Running intermittent faults and lookups for %s\n", duration)
	time.Sleep(duration)
	close(stopCh)

	// Wait for all goroutines to finish
	wg.Wait()
	fmt.Println("Case 2 simulation completed.")
}

func simulateLookup(nodes []ChordNode) {
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

	// Randomly pick a domain to lookup
	randomData := testData[rand.Intn(len(testData))]
	hashedKey := chord.HashKey(randomData.domain).String()
	randomNode := nodes[rand.Intn(len(nodes))]

	// Perform the lookup
	value, err := randomNode.node.Get(hashedKey)
	if err != nil {
		fmt.Printf("[Lookup] Failed to resolve domain '%s': %v\n", randomData.domain, err)
		return
	}

	if string(value) == randomData.ip {
		fmt.Printf("[Lookup] Successfully resolved domain '%s' to IP '%s'\n", randomData.domain, value)
	} else {
		fmt.Printf("[Lookup] Value mismatch for domain '%s': Expected '%s', Got '%s'\n", randomData.domain, randomData.ip, value)
	}
}

func introduceFault(nodes []ChordNode) {
	randomNodeIndex := rand.Intn(len(nodes))
	randomNode := nodes[randomNodeIndex]

	fmt.Printf("[Fault] Simulating fault on Node %d (ID: %s)\n", randomNodeIndex+1, randomNode.node.ID.String())

	// Simulate node going offline
	randomNode.node.IsAlive = false
	time.Sleep(time.Second * 3)

	// Bring node back online
	randomNode.node.IsAlive = true
	fmt.Printf("[Fault] Node %d (ID: %s) is back online\n", randomNodeIndex+1, randomNode.node.ID.String())
}
