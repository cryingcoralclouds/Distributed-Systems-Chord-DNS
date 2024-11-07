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
		// Start finger table maintenance for the first node
		n.startFixFingers()
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

	// Initialize finger table
	if err := n.initFingerTable(introducer); err != nil {
		return fmt.Errorf("failed to initialize finger table: %w", err)
	}

	n.startFixFingers()

	// Immediately notify our successor
	self := &RemoteNode{
		ID:      n.ID,
		Address: n.Address,
		Client:  n.Client,
	}

	if err := n.Successors[0].Client.Notify(ctx, self); err != nil {
		log.Printf("[Join] Failed to notify successor during join: %v", err)
	}
	// Start finger table maintenance after initialization
	n.startFixFingers()

	return nil
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
	stabilizeTicker := time.NewTicker(StabilizeInterval)
	fingerTicker := time.NewTicker(StabilizeInterval * 2) // Fix fingers less frequently

	go func() {
		for {
			select {
			case <-stabilizeTicker.C:
				n.stabilize()
			case <-fingerTicker.C:
				n.startFixFingers()
			case <-n.ctx.Done():
				stabilizeTicker.Stop()
				fingerTicker.Stop()
				return
			}
		}
	}()
}

// checkPredecessor checks if the predecessor is alive and updates the node's state accordingly.
func (n *Node) checkPredecessor() {
	if n.Predecessor == nil {
		return // No predecessor to check
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
		start := n.ID.Add(n.ID, new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(i-1)), nil))
		successor := n.findSuccessorInternal(start)

		log.Printf("n %d", n.ID)
		log.Printf("start: %d", start)

		if successor != nil {
			n.FingerTable[i] = successor
		}
	}
}
