package chord

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
)

// NodeClient defines the interface for remote node communication
type NodeClient interface {
	StoreKey(ctx context.Context, key string, metadata KeyMetadata) error
	GetKey(ctx context.Context, key string) ([]byte, int64, error)
	StoreReplica(ctx context.Context, key string, metadata KeyMetadata) error
	DeleteKey(ctx context.Context, key string, metadata KeyMetadata) error

	FindSuccessor(ctx context.Context, id *big.Int) (*RemoteNode, error)
	GetPredecessor(ctx context.Context) (*RemoteNode, error)
	GetSuccessors(ctx context.Context) ([]*RemoteNode, error)

	Notify(ctx context.Context, node *RemoteNode) error
	TransferKeys(ctx context.Context, start, end *big.Int) (map[string][]byte, error)
	Ping(ctx context.Context) error
}

// RemoteNode represents a reference to a remote node in the Chord ring
type RemoteNode struct {
	ID      *big.Int   `json:"id"`
	Address string     `json:"address"`
	Client  NodeClient `json:"-"` // Exclude from JSON serialization
	IsAlive bool
}

// Custom JSON marshaling for big.Int
func (n *RemoteNode) MarshalJSON() ([]byte, error) {
	type Alias RemoteNode
	return json.Marshal(&struct {
		ID string `json:"id"`
		*Alias
	}{
		ID:    n.ID.String(),
		Alias: (*Alias)(n),
	})
}

// Custom JSON unmarshaling for big.Int
func (n *RemoteNode) UnmarshalJSON(data []byte) error {
	type Alias RemoteNode
	aux := &struct {
		ID string `json:"id"`
		*Alias
	}{
		Alias: (*Alias)(n),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	id, ok := new(big.Int).SetString(aux.ID, 10)
	if !ok {
		return fmt.Errorf("invalid ID format")
	}
	n.ID = id

	// Create a new client for the remote node
	n.Client = NewHTTPNodeClient(n.Address)

	return nil
}
