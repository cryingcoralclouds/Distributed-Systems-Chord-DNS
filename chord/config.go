package chord

import (
	"errors"
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
