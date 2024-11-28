package chord

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"time"
)

const CheckPredecessorInterval = time.Second * 5 // Set your desired interval

// Node represents a node in the Chord ring
type Node struct {
	ID                *big.Int
	Address           string
	IsAlive           bool
	Successors        []*RemoteNode
	Predecessor       *RemoteNode
	FingerTable       [M]*RemoteNode
	DHT               map[string][]byte
	Versions          map[string]int64
	ReplicationStatus map[string][]*RemoteNode

	ctx     context.Context
	cancel  context.CancelFunc
	msgChan chan NodeMsg

	// Client to communicate with other nodes
	Client NodeClient
}

func init() {
	// Seed the random number generator once at the package level
	rand.Seed(time.Now().UnixNano())
}

func NewNode(addr string, client NodeClient) (*Node, error) {
	if addr == "" {
		return nil, errors.New("network address cannot be empty")
	}

	ctx, cancel := context.WithCancel(context.Background())
	node := &Node{
		ID:                HashKey(addr),
		Address:           addr,
		IsAlive:           true,
		Successors:        make([]*RemoteNode, NumSuccessors),
		DHT:               make(map[string][]byte),
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
	go node.startFixFingers()

	return node, nil
}

/*
Put:
Finds the responsible node for the hashed key.
If current node is responsible, store locally. Otherwise, forward req with key-value to the responsible node.
*/
func (n *Node) Put(hashedKey string, value []byte) error {
	if !n.IsAlive {
		return ErrNodeDown
	}

	// Convert hashedKey to *big.Int
	hash := new(big.Int)
	if _, ok := hash.SetString(hashedKey, 10); !ok {
		return fmt.Errorf("invalid hashed key: %s", hashedKey)
	}

	// Find the responsible node for the hashed key
	responsible := n.FindResponsibleNode(hash)

	ctx, cancel := context.WithTimeout(n.ctx, time.Second*2)
	defer cancel()

	// If we are the responsible node, store locally
	if responsible.ID.Cmp(n.ID) == 0 {
		n.DHT[hashedKey] = value
		n.Versions[hashedKey] = time.Now().UnixNano()
		log.Printf("Stored record locally: key=%s, value=%s", hashedKey, string(value))
		return nil
	}

	// Otherwise, forward the request to the responsible node
	err := responsible.Client.StoreKey(ctx, hashedKey, value)
	if err != nil {
		return fmt.Errorf("failed to forward record to responsible node: %w", err)
	}

	log.Printf("Forwarded record: key=%s, value=%s", hashedKey, string(value))
	return nil
}

/*
replicateKey method:
Attempts to store a key-value pair in the successor nodes.
Ignores the primary node and counts how many replicas were successfully stored.
Updates the ReplicationStatus map with the successful replicas.
*/
func (n *Node) replicateKey(primary *RemoteNode, key string, value []byte) error {
	if len(n.Successors) < ReplicationFactor {
		return fmt.Errorf("not enough successors for replication: need %d, have %d", ReplicationFactor, len(n.Successors))
	}

	replicaCount := 0
	n.ReplicationStatus[key] = []*RemoteNode{} // Initialize or reset replication status for key

	for i := 0; i < len(n.Successors) && replicaCount < ReplicationFactor; i++ {
		if n.Successors[i] == nil || n.Successors[i].ID.Cmp(primary.ID) == 0 {
			continue
		}

		ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
		err := n.Successors[i].Client.StoreKey(ctx, key, value)
		cancel()

		if err == nil {
			n.ReplicationStatus[key] = append(n.ReplicationStatus[key], n.Successors[i])
			replicaCount++
		}

	}

	if replicaCount < ReplicationFactor {
		return fmt.Errorf("failed to achieve desired replication factor: got %d, want %d", replicaCount, ReplicationFactor)
	}

	return nil
}

/*
Get:
Use the Chord ring structure to find the successor node responsible for that key hash.
Use "between" checks to confirm if the node is responsible for the key.
Retrieve the value if the node holds it; otherwise, forward the request to the responsible node.
Return value.
*/
func (n *Node) Get(hashedKey string) ([]byte, error) {
	if !n.IsAlive {
		return nil, ErrNodeDown
	}

	// Convert hashedKey to *big.Int
	hash := new(big.Int)
	if _, ok := hash.SetString(hashedKey, 10); !ok {
		return nil, fmt.Errorf("invalid hashed key: %s", hashedKey)
	}

	// Find the responsible node for the hashed key
	responsible := n.FindResponsibleNode(hash)

	// If we are the responsible node, return locally stored value
	if responsible.ID.Cmp(n.ID) == 0 {
		value, exists := n.DHT[hashedKey]
		if !exists {
			return nil, ErrKeyNotFound
		}
		log.Printf("Retrieved record locally: key=%s, value=%s", hashedKey, string(value))
		return value, nil
	}

	// Otherwise, forward the request to the responsible node
	ctx, cancel := context.WithTimeout(n.ctx, time.Second*2)
	defer cancel()

	value, _, err := responsible.Client.GetKey(ctx, hashedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve record from responsible node: %w", err)
	}

	log.Printf("Retrieved record from responsible node: key=%s, value=%s", hashedKey, string(value))
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

	// Retry once if no value was found
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
		return n.InitFingerTable(nil)
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

	return n.InitFingerTable(introducer)
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
	for key, value := range n.DHT {
		hash := HashKey(key)
		if Between(hash, start, end, true) {
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

func (n *Node) StartFixFingers() {
	go func() {
		ticker := time.NewTicker(time.Second * 5) // Adjust interval as needed
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				n.fixFingers()
			case <-n.ctx.Done():
				return
			}
		}
	}()
}

func (n *Node) fixFingers() {
	for i := 1; i < len(n.FingerTable); i++ { // temporarily change from len(n.FingerTable) to 5
		// Find the start of the ith finger
		start := new(big.Int).Add(n.ID, new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(i-1)), nil))
		successor, err := n.Client.FindSuccessor(n.ctx, start)

		log.Printf("n %d", n.ID)
		log.Printf("start: %d", start)

		if err != nil {
			log.Printf("Error fixing finger %d: %v", i, err)
			continue
		}

		if successor != nil {
			n.FingerTable[i] = successor
		}
	}
}

// ReplicatedPut handles putting a value with replication
func (n *Node) ReplicatedPut(hashedKey string, value []byte) error {
    if !n.IsAlive {
        return ErrNodeDown
    }

    // Convert hashedKey to *big.Int
    hash := new(big.Int)
    if _, ok := hash.SetString(hashedKey, 10); !ok {
        return fmt.Errorf("invalid hashed key: %s", hashedKey)
    }

    // Find primary node
    primary := n.FindResponsibleNode(hash)
    
    // Store on primary
    ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
    defer cancel()
    
    if primary.ID.Cmp(n.ID) == 0 {
        // We are primary, store locally
        n.DHT[hashedKey] = value
        n.Versions[hashedKey] = time.Now().UnixNano()
        
        // Replicate to successors
        successCount := 0
        for i := 0; i < len(n.Successors) && successCount < ReplicationFactor-1; i++ {
            if n.Successors[i] != nil && n.Successors[i].ID.Cmp(n.ID) != 0 {
                err := n.Successors[i].Client.StoreKey(ctx, hashedKey, value)
                if err == nil {
                    successCount++
                    // Track replication status
                    n.ReplicationStatus[hashedKey] = append(n.ReplicationStatus[hashedKey], n.Successors[i])
                }
            }
        }
        
        if successCount < ReplicationFactor-1 {
            log.Printf("Warning: Only achieved %d replicas out of %d desired", successCount+1, ReplicationFactor)
        }
        
        return nil
    }

    // Forward to primary
    return primary.Client.StoreKey(ctx, hashedKey, value)
}

// CheckReplication verifies and repairs replication status
func (n *Node) CheckReplication() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-n.ctx.Done():
            return
        case <-ticker.C:
            if !n.IsAlive {
                continue
            }

            for key, value := range n.DHT {
                replicas := n.ReplicationStatus[key]
                if len(replicas) < ReplicationFactor-1 {
                    // Need to create more replicas
                    ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
                    successCount := len(replicas)
                    
                    // Try to replicate to successors
                    for i := 0; i < len(n.Successors) && successCount < ReplicationFactor-1; i++ {
                        if n.Successors[i] != nil && n.Successors[i].ID.Cmp(n.ID) != 0 {
                            // Check if already a replica
                            isReplica := false
                            for _, replica := range replicas {
                                if replica.ID.Cmp(n.Successors[i].ID) == 0 {
                                    isReplica = true
                                    break
                                }
                            }
                            
                            if !isReplica {
                                err := n.Successors[i].Client.StoreKey(ctx, key, value)
                                if err == nil {
                                    successCount++
                                    n.ReplicationStatus[key] = append(n.ReplicationStatus[key], n.Successors[i])
                                }
                            }
                        }
                    }
                    cancel()
                }
            }
        }
    }
}
