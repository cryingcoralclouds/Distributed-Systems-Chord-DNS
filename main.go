package main

import (
	"chord_dns/chord"
	"chord_dns/dns"
	"fmt"
	"log"
	"time"
)

const (
	basePort = 8001
	numNodes = 10
)

type ChordNode struct {
	node   *chord.Node
	server *chord.HTTPNodeServer
}

func main() {
	// Process DNS data
	inputFile := "dns/dns_data.json"
	outputFile := "dns/hashed_dns_data.json"
	processDNS(inputFile, outputFile)

	// Initialize Chord nodes
	nodes := initializeNodes()

	// Wait for servers to start
	time.Sleep(2 * time.Second)

	// Run test suite
	runTestSuite(nodes)

	fmt.Println("\nServers running. Press Ctrl+C to exit.")
	select {}
}

func processDNS(inputFile, outputFile string) {
	err := dns.ProcessDNSData(inputFile, outputFile)
	if err != nil {
		log.Fatalf("Failed to process DNS data: %v", err)
	}
	fmt.Println("Hashed DNS data written to", outputFile)
}

func initializeNodes() []ChordNode {
	nodes := make([]ChordNode, numNodes)
	for i := 0; i < numNodes; i++ {
		addr := fmt.Sprintf(":%d", basePort+i)
		node, server := createNode(addr)
		nodes[i] = ChordNode{node: node, server: server}
		startServer(server, addr)
	}
	return nodes
}

func testFingerTable(nodes []*chord.Node) {
	fmt.Println("Testing finger table maintenance...")

	// Wait for finger tables to be populated
	time.Sleep(5 * time.Second)

	// Print finger table entries for all nodes
	for _, node := range nodes {
		fmt.Printf("\nNode %d Finger Table:\n", node.ID)
		for j, finger := range node.FingerTable {
			if finger != nil {
				fmt.Printf("Entry %d -> Node %s (Address: %s)\n",
					j, finger.ID, finger.Address)
			} else {
				fmt.Printf("Entry %d -> nil\n", j)
			}
		}
	}
}
