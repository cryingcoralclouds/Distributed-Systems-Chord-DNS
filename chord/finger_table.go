package chord

import (
	"fmt"
	"math/big"
)

/*
findSuccessorInternal method:
Finds the successor of a given ID.
Checks if the ID is between the node’s ID and the first successor’s ID.
If not, it asks the closest preceding node for the successor.
*/
func (n *Node) findSuccessorInternal(id *big.Int) *RemoteNode {
	if between(id, n.ID, n.Successors[0].ID, true) {
		return n.Successors[0]
	}

	pred := n.closestPrecedingNode(id)
	if pred == nil {
		return n.Successors[0]
	}

	succ, err := pred.Client.FindSuccessor(n.ctx, id)
	if err != nil {
		return n.Successors[0]
	}
	return succ
}

// Returns the closest preceding node in the finger table for a given ID, which helps in routing requests.
func (n *Node) closestPrecedingNode(id *big.Int) *RemoteNode {
	for i := M - 1; i >= 0; i-- {
		if n.FingerTable[i] != nil && between(n.FingerTable[i].ID, n.ID, id, false) {
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

		successor, err := introducer.Client.FindSuccessor(n.ctx, start)
		if err != nil {
			return fmt.Errorf("failed to initialize finger table at position %d: %w", i, err)
		}
		n.FingerTable[i] = successor
	}

	return nil
}