package chord

import (
    "context"
    "math/big"
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