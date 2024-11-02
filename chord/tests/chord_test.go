package chord

import (
	"context"
	"math/big"
	"testing"
	"time"
)

// Enhanced MockNodeClient for testing
type MockNodeClient struct {
	storage map[string][]byte
}

func NewMockNodeClient() *MockNodeClient {
	return &MockNodeClient{
		storage: make(map[string][]byte),
	}
}

func (m *MockNodeClient) StoreKey(ctx context.Context, key string, value []byte) error {
	m.storage[key] = value
	return nil
}

func (m *MockNodeClient) GetKey(ctx context.Context, key string) ([]byte, int64, error) {
	value, exists := m.storage[key]
	if !exists {
		return nil, 0, ErrKeyNotFound
	}
	return value, time.Now().UnixNano(), nil
}

func (m *MockNodeClient) DeleteKey(ctx context.Context, key string, version int64) error {
	delete(m.storage, key)
	return nil
}

func (m *MockNodeClient) FindSuccessor(ctx context.Context, id *big.Int) (*RemoteNode, error) {
	return nil, nil
}

func (m *MockNodeClient) GetPredecessor(ctx context.Context) (*RemoteNode, error) {
	return nil, nil
}

func (m *MockNodeClient) Notify(ctx context.Context, node *RemoteNode) error {
	return nil
}

func (m *MockNodeClient) TransferKeys(ctx context.Context, start, end *big.Int) (map[string][]byte, error) {
	return nil, nil
}

func (m *MockNodeClient) Ping(ctx context.Context) error {
	return nil
}

func TestNewNode(t *testing.T) {
	client := NewMockNodeClient()
	node, err := NewNode("localhost:8080", client)
	
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	if node == nil {
		t.Error("Expected node to be created, got nil")
	}
	
	if !node.IsAlive {
		t.Error("Expected new node to be alive")
	}
}

func TestHashKey(t *testing.T) {
	hash1 := hashKey("test1")
	hash2 := hashKey("test1")
	hash3 := hashKey("test2")
	
	if hash1.Cmp(hash2) != 0 {
		t.Error("Same input should produce same hash")
	}
	
	if hash1.Cmp(hash3) == 0 {
		t.Error("Different inputs should produce different hashes")
	}
}

func TestBetween(t *testing.T) {
	// Test cases for between function
	tests := []struct {
		id        *big.Int
		start     *big.Int
		end       *big.Int
		inclusive bool
		want      bool
	}{
		{big.NewInt(5), big.NewInt(1), big.NewInt(10), true, true},
		{big.NewInt(5), big.NewInt(1), big.NewInt(10), false, true},
		{big.NewInt(1), big.NewInt(1), big.NewInt(10), true, true},
		{big.NewInt(1), big.NewInt(1), big.NewInt(10), false, false},
	}

	for i, tt := range tests {
		got := between(tt.id, tt.start, tt.end, tt.inclusive)
		if got != tt.want {
			t.Errorf("test %d: between(%v, %v, %v, %v) = %v; want %v",
				i, tt.id, tt.start, tt.end, tt.inclusive, got, tt.want)
		}
	}
}

func TestPutAndGetLocalNode(t *testing.T) {
	client := NewMockNodeClient()
	node, _ := NewNode("localhost:8080", client)
	
	// Setup node as its own successor
	node.Successors[0] = &RemoteNode{
		ID:      node.ID,
		Address: node.Address,
		Client:  client,
	}

	// Test Put
	err := node.Put("testKey", []byte("testValue"))
	if err != nil {
		t.Errorf("Put failed: %v", err)
	}

	// Test Get
	value, err := node.Get("testKey")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	if string(value) != "testValue" {
		t.Errorf("Expected value 'testValue', got '%s'", string(value))
	}
}

func TestPutAndGetRemoteNode(t *testing.T) {
	// Create two nodes with their clients
	client1 := NewMockNodeClient()
	client2 := NewMockNodeClient()
	
	node1, _ := NewNode("localhost:8080", client1)
	node2, _ := NewNode("localhost:8081", client2)

	// Setup node2 as successor of node1
	node1.Successors[0] = &RemoteNode{
		ID:      node2.ID,
		Address: node2.Address,
		Client:  client2,
	}

	// Test Put to remote node
	err := node1.Put("testKey", []byte("testValue"))
	if err != nil {
		t.Errorf("Remote Put failed: %v", err)
	}

	// Test Get from remote node
	value, err := node1.Get("testKey")
	if err != nil {
		t.Errorf("Remote Get failed: %v", err)
	}

	if string(value) != "testValue" {
		t.Errorf("Expected value 'testValue', got '%s'", string(value))
	}
}

func TestGetNonExistentKey(t *testing.T) {
	client := NewMockNodeClient()
	node, _ := NewNode("localhost:8080", client)
	
	// Setup node as its own successor
	node.Successors[0] = &RemoteNode{
		ID:      node.ID,
		Address: node.Address,
		Client:  client,
	}

	_, err := node.Get("nonexistentKey")
	if err != ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}
}

func TestPutToDeadNode(t *testing.T) {
	client := NewMockNodeClient()
	node, _ := NewNode("localhost:8080", client)
	node.IsAlive = false

	err := node.Put("testKey", []byte("testValue"))
	if err != ErrNodeDown {
		t.Errorf("Expected ErrNodeDown, got %v", err)
	}
}
