package main

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// Configuration constants
const (
	M                 = 160 // Number of bits in the identifier space (SHA-1)
	NumSuccessors     = 3   // Number of successors for fault tolerance
	ReplicationFactor = 3   // Number of replicas for each key
)

// Timing constants
const (
	StabilizeInterval  = 1 * time.Second
	FixFingersInterval = 2 * time.Second
	CheckPredInterval  = 3 * time.Second
	NetworkTimeout     = 2 * time.Second
)

// Ring size represents the total number of unique IDs (2^160 for SHA-1)
var RingSize = new(big.Int).Exp(big.NewInt(2), big.NewInt(M), nil)

// Common errors
var (
	ErrNodeNotFound = errors.New("node not found")
	ErrKeyNotFound  = errors.New("key not found")
	ErrNodeDown     = errors.New("node is not alive")
	ErrTimeout      = errors.New("operation timed out")
)

// NodeClient defines the interface for remote node communication
type NodeClient interface {
	StoreKey(ctx context.Context, key string, value []byte) error
	GetKey(ctx context.Context, key string) ([]byte, int64, error)
	DeleteKey(ctx context.Context, key string, version int64) error

	FindSuccessor(ctx context.Context, id *big.Int) (*RemoteNode, error)
	GetPredecessor(ctx context.Context) (*RemoteNode, error)

	Notify(ctx context.Context, node *RemoteNode) error
	TransferKeys(ctx context.Context, start, end *big.Int) (map[string][]byte, error)
	Ping(ctx context.Context) error
}

// RemoteNode represents a reference to a remote node in the Chord ring
type RemoteNode struct {
	ID      *big.Int
	Address string
	Client  NodeClient
}

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

// NodeMsg represents messages between nodes
type NodeMsg struct {
	Type    string
	Payload interface{}
	Reply   chan interface{}
}

// NewNode creates a new Chord node
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
		Client:            client, // Assign the client
	}

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

	ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
	defer cancel()

	if err := responsible.Client.StoreKey(ctx, key, value); err != nil {
		return fmt.Errorf("failed to store key: %w", err)
	}

	return n.replicateKey(responsible, key, value)
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

/*
Get method:
Retrieves a value for a given key.
Similar to Put, it finds the responsible node but checks the replicas if the initial retrieval fails.
*/
func (n *Node) Get(key string) ([]byte, error) {
	if !n.IsAlive {
		return nil, ErrNodeDown
	}

	responsible := n.findSuccessorInternal(hashKey(key))

	ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
	defer cancel()

	value, _, err := responsible.Client.GetKey(ctx, key)
	if err == nil {
		return value, nil
	}

	return n.getFromReplicas(key)
}

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
		n.Predecessor = nil
		n.Successors[0] = &RemoteNode{ID: n.ID, Address: n.Address}
		return nil
	}

	ctx, cancel := context.WithTimeout(n.ctx, NetworkTimeout)
	defer cancel()

	successor, err := introducer.Client.FindSuccessor(ctx, n.ID)
	if err != nil {
		return fmt.Errorf("failed to find successor: %w", err)
	}

	n.Successors[0] = successor
	return n.initFingerTable(introducer)
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

// Takes a string input and returns its SHA-1 hash as a big integer. This is used for generating unique identifiers for keys and nodes.
func hashKey(input string) *big.Int {
	hash := sha1.New()
	hash.Write([]byte(input))
	return new(big.Int).SetBytes(hash.Sum(nil))
}

// A utility function that checks if a given ID falls between two other IDs in a circular manner, accounting for the ring topology of Chord.
func between(id, start, end *big.Int, inclusive bool) bool {
	if id == nil || start == nil || end == nil {
		return false
	}

	if start.Cmp(end) == 0 {
		return inclusive
	}

	if start.Cmp(end) < 0 {
		if inclusive {
			return id.Cmp(start) >= 0 && id.Cmp(end) <= 0
		}
		return id.Cmp(start) > 0 && id.Cmp(end) < 0
	}

	if inclusive {
		return id.Cmp(start) >= 0 || id.Cmp(end) <= 0
	}
	return id.Cmp(start) > 0 || id.Cmp(end) < 0
}
