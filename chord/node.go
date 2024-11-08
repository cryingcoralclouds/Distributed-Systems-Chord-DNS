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
Hashes the key to determine where it should be stored.
Finds the responsible node for the hashed key.
If current node is responsible, store locally. Otherwise, forward req with key-value to the responsible node.
*/
func (n *Node) Put(key string, value []byte) error {
	if !n.IsAlive {
		return ErrNodeDown
	}

	hash := HashKey(key)
	responsible := n.findSuccessorInternal(hash)

	ctx, cancel := context.WithTimeout(n.ctx, time.Second*2)
	defer cancel()

	// If we are the responsible node, store locally
	if responsible.ID.Cmp(n.ID) == 0 {
		n.DHT[key] = value
		n.Versions[key] = time.Now().UnixNano()

		// Attempt replication to successors
		if err := n.replicateKey(responsible, key, value); err != nil {
			log.Printf("Failed to replicate key %s: %v", key, err)
		}

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
Generate a hash for the key to identify its position on the ring.
Use the Chord ring structure to find the successor node responsible for that key hash.
Use "between" checks to confirm if the node is responsible for the key.
Retrieve the value if the node holds it; otherwise, forward the request to the responsible node.
Return value.
*/
func (n *Node) Get(key string) ([]byte, error) {
	if !n.IsAlive {
		return nil, ErrNodeDown
	}

	hash := HashKey(key)
	responsible := n.findSuccessorInternal(hash)

	// If we are the responsible node, return locally stored value
	if responsible.ID.Cmp(n.ID) == 0 {
		value, exists := n.DHT[key]
		if exists {
			return value, nil
		}

		// Attempt to retrieve the value from replicas if not found locally
		return n.getFromReplicas(key)

	}

	// Otherwise, forward to responsible node
	ctx, cancel := context.WithTimeout(n.ctx, time.Second*2)
	defer cancel()

	value, _, err := responsible.Client.GetKey(ctx, key)
	if err != nil {
		return n.getFromReplicas(key)
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
		log.Printf("[Join] No introducer provided, starting a new ring")
		// Set the node as its own predecessor and successor
		n.Predecessor = &RemoteNode{
			ID:      n.ID,
			Address: n.Address,
			Client:  n.Client,
		}
		n.Successors[0] = &RemoteNode{
			ID:      n.ID,
			Address: n.Address,
			Client:  n.Client,
		}

		// Initialize finger table for the new ring
		err := n.initFingerTable(nil) // Pass nil since no introducer exists
		if err != nil {
			return fmt.Errorf("failed to initialize finger table: %w", err)
		}

		// Start background processes like finger table updates
		n.startFixFingers()
		return nil
	}

	// For joining an existing ring:
	log.Printf("[Join] Joining ring via introducer at %s", introducer.Address)
	successor, err := introducer.Client.FindSuccessor(n.ctx, n.ID)
	if err != nil {
		return fmt.Errorf("failed to find successor: %w", err)
	}
	n.Successors[0] = successor

	// Notify the successor of the new predecessor
	err = successor.Client.Notify(n.ctx, &RemoteNode{
		ID:      n.ID,
		Address: n.Address,
		Client:  n.Client,
	})
	if err != nil {
		log.Printf("Warning: Failed to notify successor %s: %v", successor.Address, err)
	}

	// Initialize finger table for the node
	err = n.initFingerTable(introducer)
	if err != nil {
		return fmt.Errorf("failed to initialize finger table: %w", err)
	}

	// Start finger table and stabilization
	n.startFixFingers()
	n.startStabilize()
	return nil
}

func (n *Node) Notify(potentialPred *RemoteNode) {
	if n.Predecessor == nil || Between(potentialPred.ID, n.Predecessor.ID, n.ID, false) {
		n.Predecessor = potentialPred // Update predecessor
	}
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

	// Get successor's predecessor
	pred, err := n.Successors[0].Client.GetPredecessor(n.ctx)
	if err != nil {
		log.Printf("[Stabilize] Error getting successor's predecessor: %v", err)
		return
	}

	// Only update successor if pred is valid and closer
	if pred != nil && Between(pred.ID, n.ID, n.Successors[0].ID, false) {
		n.Successors[0] = pred
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
	stabilizeTicker := time.NewTicker(StabilizeInterval)

	go func() {
		for {
			select {
			case <-stabilizeTicker.C:
				n.stabilize()
			case <-n.ctx.Done():
				stabilizeTicker.Stop()
				return
			}
		}
	}()
}

// checkPredecessor checks if the predecessor is alive and updates the node's state accordingly.
func (n *Node) checkPredecessor() {
	if n.Predecessor == nil || n.Predecessor.Client == nil {
		return // No predecessor or no client to check
	}

	// Attempt to ping the predecessor
	ctx, cancel := context.WithTimeout(n.ctx, time.Second)
	defer cancel()

	err := n.Predecessor.Client.Ping(ctx)
	if err != nil {
		log.Printf("[checkPredecessor] Predecessor %s is not alive: %v", n.Predecessor.ID, err)
		n.Predecessor = nil // Update the predecessor to nil if it's not alive
	} else {
		log.Printf("[checkPredecessor] Predecessor %s is alive", n.Predecessor.ID)
	}
}

// startCheckPredecessor starts a periodic check for the predecessor's status.
func (n *Node) StartCheckPredecessor() {
	ticker := time.NewTicker(CheckPredecessorInterval) // Define your interval
	go func() {
		for {
			select {
			case <-ticker.C:
				n.checkPredecessor()
			case <-n.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (n *Node) startFixFingers() {
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
		successor := n.findSuccessorInternal(start)

		log.Printf("n %d", n.ID)
		log.Printf("start: %d", start)

		if successor != nil {
			n.FingerTable[i] = successor
		}
	}
}
