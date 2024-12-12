package main

import (
	"chord_dns/chord"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	defaultBasePort       = 8080
	introducerServiceName = "chord-introducer"
	defaultStabilizeDelay = 5 * time.Second
)

func main() {
	// Define and parse test-related flags
	isTest := flag.Bool("test", false, "Indicates if the node is for testing")
	flag.Parse()

	// Get configuration from environment variables
	isIntroducer := os.Getenv("IS_INTRODUCER") == "true"
	basePort := os.Getenv("BASE_PORT")
	if basePort == "" {
		basePort = strconv.Itoa(defaultBasePort)
	}
	nodeAddress := fmt.Sprintf("0.0.0.0:%s", basePort) // Node listens on 0.0.0.0
	serviceName := os.Getenv("HOSTNAME")               // Use Docker-assigned hostname

	// Create the node
	selfNode, err := chord.NewNode(serviceName+":"+basePort, chord.NewHTTPNodeClient(serviceName+":"+basePort))
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
	}

	if isIntroducer {
		// Initialize as the introducer (first node in the ring)
		selfNode.Initialize()
		log.Println("Node initialized as the introducer.")
	} else {
		// Join the Chord ring through the introducer
		introducerAddress := fmt.Sprintf("%s:%d", introducerServiceName, defaultBasePort)

		log.Printf("Waiting for introducer at %s to be ready...", introducerAddress)
		for {
			if err := checkIntroducerReady(introducerAddress); err == nil {
				break
			}
			log.Printf("Introducer not ready, retrying in 2 seconds...")
			time.Sleep(2 * time.Second)
		}

		// Attempt to join the Chord ring
		if err := selfNode.Join(&chord.RemoteNode{
			Address: introducerAddress,
			Client:  chord.NewHTTPNodeClient(introducerAddress),
		}); err != nil {
			log.Fatalf("Failed to join Chord ring: %v", err)
		}
		log.Printf("Node %s joined the Chord ring via introducer %s", serviceName, introducerAddress)
	}

	// Start the HTTP server for this node
	server := chord.NewHTTPNodeServer(selfNode)
	go startServer(server, nodeAddress)

	// Wait for stabilization
	time.Sleep(defaultStabilizeDelay)

	// If the node is for testing, exit after setup
	if *isTest {
		log.Println("Node setup completed for testing.")
		return
	}

	// Keep the server running
	log.Printf("Node is up and running at %s", nodeAddress)
	select {}
}

func checkIntroducerReady(address string) error {
	url := fmt.Sprintf("http://%s/ping", address)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("introducer not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("introducer returned unexpected status: %d", resp.StatusCode)
	}

	return nil
}

func startServer(server *chord.HTTPNodeServer, address string) {
	if err := server.Start(address); err != nil {
		log.Fatalf("Failed to start server on %s: %v", address, err)
	}
}
