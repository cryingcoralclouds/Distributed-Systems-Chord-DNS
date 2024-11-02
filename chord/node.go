package chord

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"
)

// Node represents a node in the Chord ring
type Node struct {
	ID                *big.Int
	Address           string
	IsAlive           bool
	Successors        []*RemoteNode
	Predecessor       *RemoteNode
	FingerTable       [M]*RemoteNode
	Keys              map[string][]byte
	Versions          map[string]int64
	ReplicationStatus map[string][]*RemoteNode

	ctx     context.Context
	cancel  context.CancelFunc
	msgChan chan NodeMsg

	// Client to communicate with other nodes
	Client NodeClient
}

func NewNode(addr string, client NodeClient) (*Node, error) {
    if addr == "" {
        return nil, errors.New("network address cannot be empty")
    }

    ctx, cancel := context.WithCancel(context.Background())
    node := &Node{
        ID:                hashKey(addr),
        Address:           addr,
        IsAlive:           true,
        Successors:        make([]*RemoteNode, NumSuccessors),
        Keys:              make(map[string][]byte),
        Versions:          make(map[string]int64),
        ReplicationStatus: make(map[string][]*RemoteNode),
        ctx:               ctx,
        cancel:            cancel,
        msgChan:           make(chan NodeMsg, 10),
        Client:            client,
    }

    // Initialize first successor as self
    node.Successors[0] = &RemoteNode{
        ID:      node.ID,
        Address: node.Address,
        Client:  client,
    }

	// Start stabilization
	node.startStabilize()

    return node, nil
}

/*
Put method:
Stores a key-value pair in the DHT.
Checks if the node is alive and finds the responsible node for the key.
Calls the responsible node’s StoreKey method and attempts to replicate the key to successors.
*/
func (n *Node) Put(key string, value []byte) error {
	if !n.IsAlive {
		return ErrNodeDown
	}

	hash := hashKey(key)
	responsible := n.findSuccessorInternal(hash)

	ctx, cancel := context.WithTimeout(n.ctx, time.Second*2)
	defer cancel()

	// If we are the responsible node, store locally
	if responsible.ID.Cmp(n.ID) == 0 {
		n.Keys[key] = value
		n.Versions[key] = time.Now().UnixNano()
		return nil
	}

	// Otherwise, forward to responsible node
	return responsible.Client.StoreKey(ctx, key, value)
}

/*
replicateKey method:
Attempts to store a key-value pair in the successor nodes.
Ignores the primary node and counts how many replicas were successfully stored.
Updates the ReplicationStatus map with the successful replicas.
*/
func (n *Node) replicateKey(primary *RemoteNode, key string, value []byte) error {
	successors := n.Successors
	replicaCount := 0

	for i := 0; i < len(successors) && replicaCount < ReplicationFactor; i++ {
		if successors[i].ID.Cmp(primary.ID) == 0 {
			continue
		}

		ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
		err := successors[i].Client.StoreKey(ctx, key, value)
		cancel()

		if err != nil {
			continue
		}
		replicaCount++
	}

	if replicaCount < ReplicationFactor {
		return fmt.Errorf("failed to achieve desired replication factor: got %d, want %d", replicaCount, ReplicationFactor)
	}

	n.ReplicationStatus[key] = successors[:replicaCount]
	return nil
}

/*
Get method:
Retrieves a value for a given key.
Similar to Put, it finds the responsible node but checks the replicas if the initial retrieval fails.
*/
func (n *Node) Get(key string) ([]byte, error) {
	if !n.IsAlive {
		return nil, ErrNodeDown
	}

	hash := hashKey(key)
	responsible := n.findSuccessorInternal(hash)

	// If we are the responsible node, return locally stored value
	if responsible.ID.Cmp(n.ID) == 0 {
		value, exists := n.Keys[key]
		if !exists {
			return nil, ErrKeyNotFound
		}
		return value, nil
	}

	// Otherwise, forward to responsible node
	ctx, cancel := context.WithTimeout(n.ctx, time.Second*2)
	defer cancel()

	value, _, err := responsible.Client.GetKey(ctx, key)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Save replicateKey for later on
// func (n *Node) replicateKey(primary *RemoteNode, key string, value []byte) error {
//		// To be implemented
//     return nil
// }

// Attempts to fetch the value from the replicas if the responsible node cannot be contacted.
func (n *Node) getFromReplicas(key string) ([]byte, error) {
	replicas, exists := n.ReplicationStatus[key]
	if !exists {
		return nil, ErrKeyNotFound
	}

	for _, replica := range replicas {
		ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
		value, _, err := replica.Client.GetKey(ctx, key)
		cancel()

		if err == nil {
			return value, nil
		}
	}

	return nil, ErrKeyNotFound
}

/*
Join method:
Allows a node to join the Chord ring.
Communicates with an introducer node to find its successor and initializes its finger table.
*/
func (n *Node) Join(introducer *RemoteNode) error {
    if introducer == nil {
        log.Printf("[Join] No introducer provided, starting new ring")
        n.Predecessor = nil
        n.Successors[0] = &RemoteNode{
            ID:      n.ID,
            Address: n.Address,
            Client:  n.Client,
        }
        return nil
    }
    
    ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
    defer cancel()

    // Find our successor through the introducer
    successor, err := introducer.Client.FindSuccessor(ctx, n.ID)
    if err != nil {
        return fmt.Errorf("failed to find successor: %w", err)
    }
    
    // Clear our predecessor - it will be set through stabilization
    n.Predecessor = nil
    
    // If the successor is the introducer and we're smaller, we should be its predecessor
	if successor.ID.Cmp(introducer.ID) == 0 && n.ID.Cmp(introducer.ID) < 0 {
		log.Printf("[Join] We should be the predecessor of the introducer")
	}

	// Set the successor
	n.Successors[0] = successor

    // Immediately notify our successor
    self := &RemoteNode{
        ID:      n.ID,
        Address: n.Address,
        Client:  n.Client,
    }
    
    if err := n.Successors[0].Client.Notify(ctx, self); err != nil {
        log.Printf("[Join] Failed to notify successor during join: %v", err)
    }
    
    return n.initFingerTable(introducer)
}

/*
Leave method:
Handles the removal of a node from the ring.
Transfers its keys to its predecessor and marks itself as not alive.
*/
func (n *Node) Leave() error {
	if !n.IsAlive {
		return ErrNodeDown
	}

	if n.Successors[0] != nil && n.Predecessor != nil {
		ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
		defer cancel()

		keys, err := n.TransferKeys(n.Predecessor.ID, n.ID)
		if err != nil {
			return fmt.Errorf("failed to transfer keys: %w", err)
		}

		for key, value := range keys {
			if err := n.Predecessor.Client.StoreKey(ctx, key, value); err != nil {
				return fmt.Errorf("failed to store transferred key %s: %w", key, err)
			}
		}
	}

	n.IsAlive = false
	n.cancel()
	return nil
}

// Collects keys that belong to the node and prepares them for transfer to another node.
func (n *Node) TransferKeys(start, end *big.Int) (map[string][]byte, error) {
	keys := make(map[string][]byte)
	for key, value := range n.Keys {
		hash := hashKey(key)
		if between(hash, start, end, true) {
			keys[key] = value
		}
	}

	return keys, nil
}

/*
Periodically queries successor for its predecessor,
updates successor if needed,
notifies the successor about this node's presence
*/
func (n *Node) stabilize() {
    if !n.IsAlive || n.Successors[0] == nil {
        return
    }

    // If we're our own successor and there's a predecessor, make it our successor
    if n.Successors[0].ID.Cmp(n.ID) == 0 && n.Predecessor != nil && 
       n.Predecessor.ID.Cmp(n.ID) != 0 {
        n.Successors[0] = n.Predecessor
        return
    }

    // Get successor's predecessor
    pred, err := n.Successors[0].Client.GetPredecessor(n.ctx)
    if err != nil {
        log.Printf("[Stabilize] Error getting successor's predecessor: %v", err)
        return
    }

    if pred != nil {
        
        // Update our successor if needed
        if pred.ID.Cmp(n.ID) != 0 && pred.ID.Cmp(n.Successors[0].ID) != 0 {
            n.Successors[0] = pred
        }
    }

    // Create RemoteNode for self
    self := &RemoteNode{
        ID:      n.ID,
        Address: n.Address,
        Client:  n.Client,
    }

    // Notify our successor
    if err := n.Successors[0].Client.Notify(n.ctx, self); err != nil {
        log.Printf("[Stabilize] Error notifying successor: %v", err)
    }
}

// To loop stabilization
func (n *Node) startStabilize() {
    ticker := time.NewTicker(StabilizeInterval)
    go func() {
        for {
            select {
            case <-ticker.C:
                n.stabilize()
            case <-n.ctx.Done():
                ticker.Stop()
                return
            }
        }
    }()
}
