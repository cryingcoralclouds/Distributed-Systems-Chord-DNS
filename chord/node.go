package chord

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"
)

const CheckPredecessorInterval = time.Second * 5 // Set your desired interval

type KeyMetadata struct {
	Value       []byte
	Version     int64
	IsPrimary   bool
	PrimaryNode *RemoteNode // Store who is the primary node for this key
}

// Node represents a node in the Chord ring
type Node struct {
	ID                *big.Int
	Address           string
	IsAlive           bool
	Successors        []*RemoteNode
	Predecessor       *RemoteNode
	FingerTable       [M]*RemoteNode
	DHT               map[string]KeyMetadata
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
		ID:                HashKey(addr),
		Address:           addr,
		IsAlive:           true,
		Successors:        make([]*RemoteNode, NumSuccessors),
		DHT:               make(map[string]KeyMetadata),
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
	node.StartStabilize()
	// go node.startFixFingers()

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

	// Create metadata
	metadata := KeyMetadata{
		Value:       value,
		Version:     time.Now().UnixNano(),
		IsPrimary:   responsible.ID.Cmp(n.ID) == 0,
		PrimaryNode: responsible,
	}

	// If we are the responsible node, store locally
	if responsible.ID.Cmp(n.ID) == 0 {
		n.DHT[hashedKey] = metadata
		n.Versions[hashedKey] = metadata.Version
		log.Printf("Stored record locally: key=%s, value=%s", hashedKey, string(value))
		return nil
	}

	// Otherwise, forward the request to the responsible node
	err := responsible.Client.StoreKey(ctx, hashedKey, metadata)
	if err != nil {
		return fmt.Errorf("failed to forward record to responsible node: %w", err)
	}

	log.Printf("Forwarded record: key=%s, value=%s", hashedKey, string(value))
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
		metadata, exists := n.DHT[hashedKey]
		if !exists {
			return nil, ErrKeyNotFound
		}
		log.Printf("Retrieved record locally: key=%s, value=%s", hashedKey, string(metadata.Value))
		return metadata.Value, nil
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

	// Get successor's predecessor to determine key range
	predecessorOfSuccessor, err := successor.Client.GetPredecessor(ctx)
	if err != nil {
		return fmt.Errorf("failed to get successor's predecessor: %w", err)
	}
	// Calculate start hash (predecessor of successor's hash)
	var startHash *big.Int
	if predecessorOfSuccessor == nil {
		// If successor has no predecessor, use successor's hash
		startHash = successor.ID
	} else {
		startHash = predecessorOfSuccessor.ID
	}
	// Request key transfer for range (predecessor(successor)_hash, new_node_hash]
	keys, err := successor.Client.TransferKeys(ctx, startHash, n.ID)
	if err != nil {
		log.Printf("[Join] Warning: failed to transfer keys: %v", err)
	} else {
		// Store transferred keys
		for key, value := range keys {
			n.DHT[key] = KeyMetadata{
				Value:     value,
				Version:   time.Now().UnixNano(),
				IsPrimary: true,
				PrimaryNode: &RemoteNode{
					ID:      n.ID,
					Address: n.Address,
					Client:  n.Client,
				},
			}
		}
		log.Printf("[Join] Successfully transferred %d keys", len(keys))
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

	// Only attempt to transfer keys if we have both predecessor and successor
	if n.Successors[0] != nil && n.Predecessor != nil {
		ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
		defer cancel()

		// Transfer keys to predecessor
		keys, err := n.TransferKeys(n.Predecessor.ID, n.ID)
		if err != nil {
			return fmt.Errorf("failed to transfer keys: %w", err)
		}

		// Store transferred keys on predecessor
		for key, value := range keys {
			metadata := KeyMetadata{
				Value:       value,
				Version:     time.Now().UnixNano(),
				IsPrimary:   true,
				PrimaryNode: n.Predecessor,
			}
			if err := n.Predecessor.Client.StoreKey(ctx, key, metadata); err != nil {
				return fmt.Errorf("failed to store transferred key %s: %w", key, err)
			}
		}
		// Notify predecessor to update its successor
		if err := n.Predecessor.Client.Notify(ctx, n.Successors[0]); err != nil {
			log.Printf("Failed to notify predecessor of departure: %v", err)
		}
		// Notify successor to update its predecessor
		if err := n.Successors[0].Client.Notify(ctx, n.Predecessor); err != nil {
			log.Printf("Failed to notify successor of departure: %v", err)
		}
	}
	// Clear all internal state
	n.DHT = make(map[string]KeyMetadata)
	n.Versions = make(map[string]int64)
	n.ReplicationStatus = make(map[string][]*RemoteNode)
	n.Successors = make([]*RemoteNode, NumSuccessors)
	n.Predecessor = nil

	n.IsAlive = false
	n.cancel() // Cancel context to stop all background operations
	return nil
}

// SimulateFault makes the node non-operational without shutting down the server
func (n *Node) SimulateFault() {
	if n.IsAlive {
		n.IsAlive = false
		log.Printf("Node %s is now in a fault state.", n.ID)
		// Notify successors
		if n.Predecessor != nil {
			n.Predecessor.Client.Notify(context.TODO(), n.Successors[0])
		}
		if n.Successors[0] != nil {
			n.Successors[0].Client.Notify(context.TODO(), n.Predecessor)
		}
	}
}

// RestoreNode restores a node from the fault state
func (n *Node) RestoreNode() {
	if !n.IsAlive {
		n.IsAlive = true
		log.Printf("Node %s has been restored to an active state.", n.ID)
	}
}

// Collects keys that belong to the node and prepares them for transfer to another node.
func (n *Node) TransferKeys(start, end *big.Int) (map[string][]byte, error) {
	keys := make(map[string][]byte)
	for key, metadata := range n.DHT {
		// Convert the key string (which is already a hash) directly to big.Int
		hash := new(big.Int)
		hash.SetString(key, 10) // Parse the key as base 10 number		log.Printf("KEY AND HASH: %s, %d", key, hash)
		if metadata.IsPrimary && Between(hash, start, end, true) {
			keys[key] = metadata.Value

			// Update our copy to non-primary
			n.DHT[key] = KeyMetadata{
				Value:     metadata.Value,
				Version:   metadata.Version,
				IsPrimary: false,
				PrimaryNode: &RemoteNode{
					ID:      end, // The new node's ID
					Address: "",  // Will be updated through stabilization
					Client:  nil,
				},
			}
		}
		log.Printf("[Between] Checking if %s is between %s and %s (inclusive: %v)",
			hash.String(), start.String(), end.String(), true)
		log.Printf("TRUE OR NOT: %t", Between(hash, start, end, true))
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
		// Don't return here, continue to update successor list
	} else if pred != nil && pred.ID.Cmp(n.ID) != 0 && pred.ID.Cmp(n.Successors[0].ID) != 0 {
		// Update our immediate successor if needed
		n.Successors[0] = pred
	}

	// Get successor list from our immediate successor
	ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
	defer cancel()

	successorList, err := n.Successors[0].Client.GetSuccessors(ctx)
	if err != nil {
		log.Printf("[Stabilize] Error getting successor list: %v", err)
	} else {
		// Update our successor list
		// Keep our immediate successor (at index 0)
		// Copy successor's list into our list starting at index 1
		for i := 1; i < len(n.Successors); i++ {
			if i-1 < len(successorList) {
				n.Successors[i] = successorList[i-1]
			} else {
				n.Successors[i] = nil
			}
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

// Add method to get successors list
func (n *Node) GetSuccessors() []*RemoteNode {
	if !n.IsAlive {
		return nil
	}
	return n.Successors
}

// To loop stabilization
func (n *Node) StartStabilize() {
	ticker := time.NewTicker(StabilizeInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				if n.IsAlive {
					n.stabilize()
					n.startFixFingers()
					n.CheckReplication()
				}
			case <-n.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
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

	// Create metadata
	version := time.Now().UnixNano()
	metadata := KeyMetadata{
		Value:       value,
		Version:     version,
		IsPrimary:   primary.ID.Cmp(n.ID) == 0,
		PrimaryNode: primary,
	}

	ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
	defer cancel()

	if primary.ID.Cmp(n.ID) == 0 {
		n.DHT[hashedKey] = metadata

		// Log successor list state
		for i, succ := range n.Successors {
			if succ != nil {
				continue
			} else {
				log.Printf("  Successor[%d]: nil", i)
			}
		}

		// Replicate to successors
		successCount := 0
		for i := 0; i < len(n.Successors) && successCount < ReplicationFactor-1; i++ {
			if n.Successors[i] != nil && n.Successors[i].ID.Cmp(n.ID) != 0 {

				replicaData := KeyMetadata{
					Value:       value,
					Version:     version,
					IsPrimary:   false,
					PrimaryNode: primary,
				}

				err := n.Successors[i].Client.StoreReplica(ctx, hashedKey, replicaData)
				if err == nil {
					successCount++
					n.ReplicationStatus[hashedKey] = append(n.ReplicationStatus[hashedKey], n.Successors[i])
				} else {
					log.Printf("[ReplicatedPut] Failed to replicate to successor %s: %v",
						n.Successors[i].ID, err)
				}
			} else {
				log.Printf("[ReplicatedPut] Skipping successor[%d] (nil or self)", i)
			}
		}

		if successCount < ReplicationFactor-1 {
			log.Printf("[ReplicatedPut] Warning: Only achieved %d replicas out of %d desired for key %s",
				successCount+1, ReplicationFactor, hashedKey)
		}

		return nil
	}

	return primary.Client.StoreKey(ctx, hashedKey, metadata)
}

func (n *Node) ReplicateToSuccessors(key string, value []byte, version int64) error {
	// Skip if we're not the primary node for this key
	hash := HashKey(key)
	if !Between(hash, n.Predecessor.ID, n.ID, true) {
		return fmt.Errorf("not primary node for key %s", key)
	}

	// Store locally as primary
	n.DHT[key] = KeyMetadata{
		Value:     value,
		Version:   version,
		IsPrimary: true,
		PrimaryNode: &RemoteNode{
			ID:      n.ID,
			Address: n.Address,
			Client:  n.Client,
		},
	}

	// Replicate to R-1 successors (where R is replication factor)
	successCount := 0
	for i := 0; i < len(n.Successors) && successCount < ReplicationFactor-1; i++ {
		if n.Successors[i] != nil && n.Successors[i].ID.Cmp(n.ID) != 0 {
			ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
			defer cancel()

			// Send both value and metadata
			replicaData := KeyMetadata{
				Value:     value,
				Version:   version,
				IsPrimary: false,
				PrimaryNode: &RemoteNode{
					ID:      n.ID,
					Address: n.Address,
					Client:  n.Client,
				},
			}

			err := n.Successors[i].Client.StoreReplica(ctx, key, replicaData)
			if err == nil {
				successCount++
			}
		}
	}

	return nil
}

// CheckReplication verifies and repairs replication status
func (n *Node) CheckReplication() {

	for key, metadata := range n.DHT {
		if metadata.IsPrimary {

			replicas := n.ReplicationStatus[key]

			if len(replicas) < ReplicationFactor-1 {
				log.Printf("[CheckReplication] Need to create more replicas for key %s", key)

				ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
				successCount := len(replicas)

				for i, succ := range n.Successors {
					if succ != nil {
						log.Printf("  Successor[%d]: nil", i)
					}
				}

				for i := 0; i < len(n.Successors) && successCount < ReplicationFactor-1; i++ {
					if n.Successors[i] != nil && n.Successors[i].ID.Cmp(n.ID) != 0 {
						isReplica := false
						for _, replica := range replicas {
							if replica.ID.Cmp(n.Successors[i].ID) == 0 {
								isReplica = true
								break
							}
						}

						if !isReplica {
							log.Printf("[CheckReplication] Attempting to replicate key %s to successor %s",
								key, n.Successors[i].ID)

							replicaData := KeyMetadata{
								Value:       metadata.Value,
								Version:     metadata.Version,
								IsPrimary:   false,
								PrimaryNode: metadata.PrimaryNode,
							}

							err := n.Successors[i].Client.StoreReplica(ctx, key, replicaData)
							if err == nil {
								successCount++
								n.ReplicationStatus[key] = append(n.ReplicationStatus[key], n.Successors[i])
								log.Printf("[CheckReplication] Successfully created new replica for key %s on node %s (%d/%d)",
									key, n.Successors[i].ID, successCount+1, ReplicationFactor)
							} else {
								log.Printf("[CheckReplication] Failed to create replica on node %s: %v",
									n.Successors[i].ID, err)
							}
						}
					}
				}
				cancel()
			}
		}
	}
}
