package main

import (
	"chord_dns/chord"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

func runCase1(nodes []ChordNode, concurrentRequests int) {
	printSeparator("Initialising Case 1: Concurrent Lookup Requests")
	testNodeJoining(nodes)

	// Allow time for stabilization and finger table setup
	time.Sleep(5 * time.Second)

	printSeparator("Putting Key-Value Pairs into DHTs")
	runBaseCasePut(nodes)

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

	// Perform concurrent lookup requests
	fmt.Printf("\nRunning %d concurrent lookup requests...\n\n", concurrentRequests)
	var wg sync.WaitGroup
	wg.Add(concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func(requestID int) {
			defer wg.Done()

			// Randomly pick a domain to lookup
			randomData := testData[rand.Intn(len(testData))]
			hashedKey := chord.HashKey(randomData.domain).String()
			randomNode := nodes[rand.Intn(len(nodes))]

			// Measure lookup time
			startTime := time.Now()

			// Perform the lookup
			value, err := randomNode.node.Get(hashedKey)
			elapsedTime := time.Since(startTime)

			if err != nil {
				fmt.Printf("[Request %d] Failed to lookup domain '%s': %v\n", requestID, randomData.domain, err)
				return
			}

			// Check the result
			if string(value) == randomData.ip {
				fmt.Printf("[Request %d] Successfully resolved domain '%s' to IP '%s' (Time: %s)\n", requestID, randomData.domain, value, elapsedTime)
			} else {
				fmt.Printf("[Request %d] Value mismatch for domain '%s': Expected '%s', Got '%s'\n", requestID, randomData.domain, randomData.ip, value)
			}
		}(i)
	}

	// Wait for all requests to complete
	wg.Wait()
	printSeparator("Concurrent lookup test completed.")
}
