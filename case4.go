package main

import (
	"chord_dns/chord"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"time"
)

func runCase4(nodes []ChordNode) {
	printSeparator("Case 4: Testing Key Transfer on Late Node Join")
	testNodeJoining(nodes)

	// Allow time for stabilization and finger table setup
	time.Sleep(5 * time.Second)

	printSeparator("Putting Key-Value Pairs into DHTs")
	runBaseCasePut(nodes)

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

	// 7. Print replication status to check for excess replicas
	// fmt.Println("\nChecking replication status before cleanup...")
	// printReplicationStatus(newNodes)

	// 8. Identify and delete excess replicas
	fmt.Println("\nIdentifying and deleting excess replicas...")
	cleanupExcessReplicas(newNodes)

	// 9. Print final state after cleanup
	// fmt.Println("\nDHT state after cleanup of excess replicas:")
	// testPrintDHTs(newNodes)
	fmt.Println("\nFinal replication status:")
	printReplicationStatus(newNodes)
}
func cleanupExcessReplicas(nodes []ChordNode) {
	// Map to track all unique keys in the network
	allKeys := make(map[string]bool)
	for _, node := range nodes {
		for key := range node.node.DHT {
			allKeys[key] = true
		}
	}

	// For each key, check replication status and clean up if needed
	for key := range allKeys {
		// Find primary node for this key
		keyBigInt := new(big.Int)
		keyBigInt.SetString(key, 10)
		primaryNode := nodes[0].node.FindResponsibleNode(keyBigInt)

		// Get all nodes that have this key
		var nodesWithKey []struct {
			node     *chord.Node
			metadata chord.KeyMetadata
			index    int
		}

		for i, node := range nodes {
			if metadata, exists := node.node.DHT[key]; exists {
				nodesWithKey = append(nodesWithKey, struct {
					node     *chord.Node
					metadata chord.KeyMetadata
					index    int
				}{
					node:     node.node,
					metadata: metadata,
					index:    i,
				})
			}
		}

		// If we have more copies than replication factor
		if len(nodesWithKey) > chord.ReplicationFactor {
			fmt.Printf("Key %s has %d copies (exceeds replication factor %d)\n",
				key, len(nodesWithKey), chord.ReplicationFactor)

			// Sort nodes by their distance from primary node
			// Keep primary + closest (ReplicationFactor-1) nodes as replicas
			sort.Slice(nodesWithKey, func(i, j int) bool {
				distI := new(big.Int).Sub(nodesWithKey[i].node.ID, primaryNode.ID)
				distJ := new(big.Int).Sub(nodesWithKey[j].node.ID, primaryNode.ID)
				return distI.Cmp(distJ) < 0
			})

			// Delete from excess nodes
			for i := chord.ReplicationFactor; i < len(nodesWithKey); i++ {
				nodeInfo := nodesWithKey[i]
				fmt.Printf("Deleting excess replica from Node %d (ID: %s)\n",
					nodeInfo.index+1, nodeInfo.node.ID)

				// Delete locally
				delete(nodeInfo.node.DHT, key)
				// Remove from replication status
				if status, exists := nodeInfo.node.ReplicationStatus[key]; exists {
					// Remove this node from the status
					var newStatus []*chord.RemoteNode
					for _, replica := range status {
						if replica.ID.Cmp(nodeInfo.node.ID) != 0 {
							newStatus = append(newStatus, replica)
						}
					}
					nodeInfo.node.ReplicationStatus[key] = newStatus
				}
			}
		}
	}
}
