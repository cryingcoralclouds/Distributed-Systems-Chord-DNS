package chord

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"
)

/*
findSuccessorInternal method:
Finds the successor of a given ID.
Checks if the ID is between the node’s ID and the first successor’s ID.
If not, it asks the closest preceding node for the successor.
*/
func (n *Node) findSuccessorInternal(id *big.Int) *RemoteNode {
	// If we have no successors or our successor is ourselves
	if n.Successors[0] == nil || n.Successors[0].ID.Cmp(n.ID) == 0 {
		// If the incoming node's ID is less than ours, it should be our successor
		if id.Cmp(n.ID) < 0 {
			return &RemoteNode{
				ID:      id,
				Address: n.Address, // This will be updated when the node actually joins
				Client:  n.Client,
			}
		}
		// Otherwise we're the successor
		return &RemoteNode{
			ID:      n.ID,
			Address: n.Address,
			Client:  n.Client,
		}
	}

	// Check if the ID falls between us and our successor
	if Between(id, n.ID, n.Successors[0].ID, false) {
		return n.Successors[0]
	}

	// Find closest preceding node
	pred := n.closestPrecedingNode(id)
	if pred == nil {
		return n.Successors[0]
	}

	succ, err := pred.Client.FindSuccessor(n.ctx, id)
	if err != nil {
		log.Printf("Error finding successor through predecessor: %v", err)
		return n.Successors[0]
	}
	return succ
}

func (n *Node) FindSuccessor(ctx context.Context, id *big.Int) (*RemoteNode, error) {
	if n.Successors[0] != nil && Between(id, n.ID, n.Successors[0].ID, true) {
		return n.Successors[0], nil
	}

	closest := n.closestPrecedingNode(id)
	if closest == nil || closest.Client == nil {
		log.Printf("Fallback: closest preceding node or its client is nil")
		return n.Successors[0], fmt.Errorf("closest preceding node is nil")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	successor, err := closest.Client.FindSuccessor(ctx, id)
	if err != nil {
		log.Printf("Error querying closest preceding node: %v", err)
		return n.Successors[0], fmt.Errorf("could not find successor")
	}

	return successor, nil
}

// Returns the closest preceding node in the finger table for a given ID, which helps in routing requests.
func (n *Node) closestPrecedingNode(id *big.Int) *RemoteNode {
	for i := M - 1; i >= 0; i-- {
		if n.FingerTable[i] != nil && Between(n.FingerTable[i].ID, n.ID, id, false) {
			return n.FingerTable[i]
		}
	}
	return nil
}

// Initializes the finger table entries for the node by finding successors for each entry.
func (n *Node) initFingerTable(introducer *RemoteNode) error {
	for i := 0; i < M; i++ {
		start := new(big.Int).Add(n.ID, new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(i)), RingSize))
		start.Mod(start, RingSize)

		var successor *RemoteNode
		var err error

		if introducer != nil {
			// Use the introducer to find the successor for this finger entry
			successor, err = introducer.Client.FindSuccessor(n.ctx, start)
			if err != nil {
				return fmt.Errorf("failed to initialize finger table at position %d: %w", i, err)
			}
		} else {
			// If no introducer, use the node's current successor
			successor = n.Successors[0]
		}

		n.FingerTable[i] = successor
	}

	return nil
}
