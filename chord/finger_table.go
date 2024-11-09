package chord

import (
    "context"
    "fmt"
    "log"
    "math/big"
    // "math/rand"
    "time"
)

// FingerEntry represents an entry in the finger table
type FingerEntry struct {
    Start  *big.Int    // Start of the finger interval
    Node   *RemoteNode // Node responsible for the interval
}

// InitFingerTable initializes all finger table entries when a node starts
func (n *Node) InitFingerTable(introducer *RemoteNode) error {
    if introducer == nil {        
        // Create self reference
        self := &RemoteNode{
            ID:      n.ID,
            Address: n.Address,
            Client:  n.Client,
        }

        // Initialize first finger (immediate successor is self)
        n.FingerTable[0] = self
        n.Successors[0] = self

        // Initialize remaining fingers following the same logic as other nodes
        for i := 1; i < M; i++ {
            start := calculateFingerStart(n.ID, i)
            
            // In a single-node ring, all queries for successors will lead back to self
            // But we still calculate and check the intervals to maintain consistency
            // with the protocol's finger table structure
            if Between(start, n.ID, n.ID, true) {
                n.FingerTable[i] = self
            } else {
                // In a single-node ring, this case technically won't occur
                // but we include it for protocol consistency
                n.FingerTable[i] = self
            }
        }

        // Start finger table maintenance
        go n.startFixFingers()
        
        return nil
    }

    // Rest of the implementation for nodes joining through introducer
    
    // Initialize first finger (successor)
    start := calculateFingerStart(n.ID, 0)
    successor, err := introducer.Client.FindSuccessor(n.ctx, start)
    if err != nil {
        return fmt.Errorf("failed to find first successor: %w", err)
    }
    n.FingerTable[0] = successor
    n.Successors[0] = successor

    // Initialize remaining fingers
    for i := 1; i < M; i++ {
        start := calculateFingerStart(n.ID, i)
        
        // Optimization: if this finger start falls within the range
        // of previous finger, use the same node
        if Between(start, n.ID, n.FingerTable[i-1].ID, true) {
            n.FingerTable[i] = n.FingerTable[i-1]
        } else {
            successor, err := introducer.Client.FindSuccessor(n.ctx, start)
            if err != nil {
                continue
            }
            n.FingerTable[i] = successor
        }
    }

    // Start finger table maintenance
    go n.startFixFingers()
    
    return nil
}

// Finds the successor node for a given ID using the finger table
func (n *Node) FindResponsibleNode(id *big.Int) *RemoteNode {
    // Case 1: If we're the only node in the ring
    if n.Successors[0] == nil || n.Successors[0].ID.Cmp(n.ID) == 0 {
        return &RemoteNode{
            ID:      n.ID,
            Address: n.Address,
            Client:  n.Client,
        }
    }

    // Case 2: Check if ID is between us and our immediate successor
    if Between(id, n.ID, n.Successors[0].ID, false) {
        return n.Successors[0]
    }

    // Case 3: Use finger table to find closest preceding node
    closestPred := n.GetClosestPrecedingFinger(id)
    if closestPred == nil || closestPred.ID.Cmp(n.ID) == 0 {
        // If no closer node found, return our successor
        return n.Successors[0]
    }

    // Forward the query to the closest preceding node
    ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
    defer cancel()

    successor, err := closestPred.Client.FindSuccessor(ctx, id)
    if err != nil {
        log.Printf("Error finding successor through closest preceding node: %v", err)
        // Fallback to our successor if lookup fails
        return n.Successors[0]
    }

    return successor
}

// calculateFingerStart calculates the start of the ith finger interval
func calculateFingerStart(nodeID *big.Int, i int) *big.Int {
    // start = (n + 2^i) mod 2^m
    exp := new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(i)), nil)
    sum := new(big.Int).Add(nodeID, exp)
    return new(big.Int).Mod(sum, RingSize)
}

// Keeping this just in case. Currently we are fixing finger table sequentially, but 
// the real Chord chooses random finger table entry to update, 
// which takes 40 iterations to achieve the perfect finger table for us.
// // fixFingers periodically refreshes finger table entries
// func (n *Node) fixFingers() {
//     if !n.IsAlive {
//         return
//     }

	// // Fix all fingers sequentially in one go
	// for i := 0; i < M; i++ {
    //     start := calculateFingerStart(n.ID, i)
    //     successor := n.FindResponsibleNode(start)
        
    //     if successor != nil && (n.FingerTable[i] == nil || 
    //        successor.ID.Cmp(n.FingerTable[i].ID) != 0) {
    //         n.FingerTable[i] = successor
    //     }
    // }

	//     // Pick a random finger to fix
	//     i := rand.Intn(M)
	//     start := calculateFingerStart(n.ID, i)
		
	//     successor := n.FindResponsibleNode(start)
	//     if successor != nil && successor.ID.Cmp(n.FingerTable[i].ID) != 0 {
	//         n.FingerTable[i] = successor
	//     }
// }

func (n *Node) startFixFingers() {
    // Start with first finger
    currentFinger := 0

    ticker := time.NewTicker(FixFingersInterval)
    defer ticker.Stop()

    for {
        select {
        case <-n.ctx.Done():
            return
        case <-ticker.C:
            if !n.IsAlive {
                continue
            }

            // Fix current finger
            start := calculateFingerStart(n.ID, currentFinger)
            successor := n.FindResponsibleNode(start)

            if successor != nil {
                if n.FingerTable[currentFinger] == nil || 
                   successor.ID.Cmp(n.FingerTable[currentFinger].ID) != 0 {
                    n.FingerTable[currentFinger] = successor
                }
            }

            // Move to next finger
            currentFinger = (currentFinger + 1) % M
        }
    }
}

// UpdateFingerTable updates finger table entries after a node join/leave
func (n *Node) UpdateFingerTable(updated *RemoteNode) {
    for i := 0; i < M; i++ {
        start := calculateFingerStart(n.ID, i)
        if Between(start, n.ID, updated.ID, true) {
            n.FingerTable[i] = updated
        }
    }
}

// GetClosestPrecedingFinger finds the closest finger that precedes given ID
func (n *Node) GetClosestPrecedingFinger(id *big.Int) *RemoteNode {
    for i := M - 1; i >= 0; i-- {
        if n.FingerTable[i] != nil && 
           Between(n.FingerTable[i].ID, n.ID, id, false) {
            // Verify finger is still alive
            ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
            err := n.FingerTable[i].Client.Ping(ctx)
            cancel()
            
            if err == nil {
                return n.FingerTable[i]
            }
            // If finger is dead, continue searching
        }
    }
    return n.Successors[0] // Fall back to immediate successor
}