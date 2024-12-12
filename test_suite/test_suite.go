package main

import (
	"chord_dns/chord"
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"
)

type TestConfig struct {
	RunAll          bool
	TestPing        bool
	TestStabilize   bool
	TestFingers     bool
	TestOperations  bool
	TestDHT         bool
	TestReplication bool
	TestSuccessors  bool
}

type ChordNode struct {
	node *chord.Node
}

func main() {
	// Parse test flags
	config := &TestConfig{
		RunAll:          true, // Default to running all tests
		TestPing:        os.Getenv("TEST_PING") == "true",
		TestStabilize:   os.Getenv("TEST_STABILIZE") == "true",
		TestFingers:     os.Getenv("TEST_FINGERS") == "true",
		TestOperations:  os.Getenv("TEST_OPERATIONS") == "true",
		TestDHT:         os.Getenv("TEST_DHT") == "true",
		TestReplication: os.Getenv("TEST_REPLICATION") == "true",
		TestSuccessors:  os.Getenv("TEST_SUCCESSORS") == "true",
	}

	// Discover nodes in the Chord ring
	nodes := discoverNodes()

	// Run the test suite
	runTestSuite(nodes, config)
}

func discoverNodes() []ChordNode {
    basePort := 8080
    nodes := []ChordNode{}

    // Discover nodes by iterating through a range of potential node names
    for i := 1; ; i++ {
        address := fmt.Sprintf("http://chord-node-%d:%d", i, basePort)
        client := chord.NewHTTPNodeClient(address)

        // Ping the node to verify it is running
        err := client.Ping(context.TODO())
        if err != nil {
            log.Printf("Node at %s is not reachable: %v\n", address, err)
            break // Stop discovery once we encounter a missing node
        }

        // Add the discovered node
        nodes = append(nodes, ChordNode{
            node: &chord.Node{
                Address: address,
                Client:  client,
            },
        })
    }

    if len(nodes) == 0 {
        log.Fatal("No nodes discovered. Ensure the Chord ring is running.")
    }

    log.Printf("Discovered %d nodes in the Chord ring.\n", len(nodes))
    return nodes
}

func runTestSuite(nodes []ChordNode, config *TestConfig) {
	if config.RunAll {
		runAllTests(nodes)
		return
	}

	if config.TestPing {
		testPing(nodes)
	}
	if config.TestStabilize {
		testStabilization(nodes)
	}
	if config.TestFingers {
		testFingerTables(nodes)
	}
	if config.TestOperations {
		testPutAndGet(nodes)
	}
	if config.TestDHT {
		testPrintDHTs(nodes)
	}
	if config.TestReplication {
		printReplicationStatus(nodes)
	}
	if config.TestSuccessors {
		testSuccessorLists(nodes)
	}
}

func runAllTests(nodes []ChordNode) {
	testPing(nodes)
	testStabilization(nodes)
	testFingerTables(nodes)
	testSuccessorLists(nodes)
	testPutAndGet(nodes)
	testPrintDHTs(nodes)
	printReplicationStatus(nodes)
}

func testPing(nodes []ChordNode) {
	fmt.Println("Testing node connectivity...")
	for _, node := range nodes {
		if node.node == nil {
			fmt.Printf("Node %s is not reachable\n", node.node.Address)
			continue
		}
		fmt.Printf("Node %s is reachable\n", node.node.Address)
	}
}

func testStabilization(nodes []ChordNode) {
	fmt.Println("Testing stabilization...")
	time.Sleep(5 * time.Second) // Allow time for stabilization
	for _, node := range nodes {
		fmt.Printf("Node %s: Successor -> %s, Predecessor -> %s\n",
			node.node.Address,
			getNodeID(node.node.Successors[0]),
			getNodeID(node.node.Predecessor))
	}
}

func testFingerTables(nodes []ChordNode) {
	fmt.Println("Testing finger tables...")
	for _, node := range nodes {
		fmt.Printf("Finger table for node %s:\n", node.node.Address)
		for i, finger := range node.node.FingerTable {
			fmt.Printf("  Finger %d -> %s\n", i, getNodeID(finger))
		}
	}
}

func testSuccessorLists(nodes []ChordNode) {
	fmt.Println("Testing successor lists...")
	for _, node := range nodes {
		fmt.Printf("Node %s Successor List:\n", node.node.Address)
		for i, successor := range node.node.Successors {
			fmt.Printf("  [%d] -> %s\n", i, getNodeID(successor))
		}
	}
}

func testPutAndGet(nodes []ChordNode) {
	fmt.Println("Testing Put and Get operations...")
	testData := map[string]string{
		"google.com":    "172.217.164.110",
		"facebook.com":  "157.240.22.35",
		"example.com":   "93.184.216.34",
		"openai.com":    "13.248.155.104",
	}

	for domain, ip := range testData {
		node := nodes[rand.Intn(len(nodes))].node
		key := chord.HashKey(domain).String()
		if err := node.ReplicatedPut(key, []byte(ip)); err != nil {
			fmt.Printf("Failed to store %s -> %s: %v\n", domain, ip, err)
		} else {
			fmt.Printf("Stored %s -> %s\n", domain, ip)
		}
	}
}

func testPrintDHTs(nodes []ChordNode) {
	fmt.Println("Printing DHTs...")
	for _, node := range nodes {
		fmt.Printf("Node %s DHT:\n", node.node.Address)
		for key, metadata := range node.node.DHT {
			fmt.Printf("  Key: %s, Value: %s\n", key, string(metadata.Value))
		}
	}
}

func printReplicationStatus(nodes []ChordNode) {
	fmt.Println("Checking replication status...")
	for _, node := range nodes {
		fmt.Printf("Node %s replication status:\n", node.node.Address)
		for key, replicas := range node.node.ReplicationStatus {
			fmt.Printf("  Key: %s, Replicas: %d\n", key, len(replicas))
		}
	}
}

func getNodeID(node *chord.RemoteNode) string {
	if node == nil {
		return "nil"
	}
	return node.ID.String()
}
