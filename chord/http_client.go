package chord

/*
HTTPNodeClient
* Allows a node to send HTTP req to other nodes' HTTP servers.
* Ping - to check if a remote node is alive.
* Other methods (like StoreKey, GetKey, DeleteKey, etc.) are not implemented yet.
*/

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
)

type HTTPNodeClient struct {
    client  *http.Client
    baseURL string
}

func NewHTTPNodeClient(address string) NodeClient {
    return &HTTPNodeClient{
        client:  &http.Client{},
        baseURL: fmt.Sprintf("http://localhost%s", address), // e.g. http://localhost:8001
    }
}

// Implement just the essential methods of NodeClient interface for now
func (c *HTTPNodeClient) Ping(ctx context.Context) error {
    req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/ping", c.baseURL), nil)	// Send req to http://localhost:8001/ping
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    resp, err := c.client.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    return nil
}

func (c *HTTPNodeClient) FindSuccessor(ctx context.Context, id *big.Int) (*RemoteNode, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", 
        fmt.Sprintf("%s/successor/%s", c.baseURL, id.String()), nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    var node RemoteNode
    if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    // Client will be automatically created by UnmarshalJSON
    return &node, nil
}

func (c *HTTPNodeClient) GetPredecessor(ctx context.Context) (*RemoteNode, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", 
        fmt.Sprintf("%s/predecessor", c.baseURL), nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusNotFound {
        return nil, nil // No predecessor
    }
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    var node RemoteNode
    if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    // Create a new client for the remote node
    node.Client = NewHTTPNodeClient(node.Address)
    return &node, nil
}

// The notify endpoint is necessary for stabilization to work
// Recall: stabilize() notifies the succssor about this node's presence (refer to README)
func (c *HTTPNodeClient) Notify(ctx context.Context, node *RemoteNode) error {
    // Marshal the node data
    data, err := json.Marshal(node)
    if err != nil {
        return fmt.Errorf("failed to marshal node data: %w", err)
    }

    // Create request
    req, err := http.NewRequestWithContext(ctx, "POST", 
        fmt.Sprintf("%s/notify", c.baseURL), bytes.NewBuffer(data))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    // Send request
    resp, err := c.client.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    return nil
}

// Stub implementations for other required NodeClient interface methods
func (c *HTTPNodeClient) StoreKey(ctx context.Context, key string, value []byte) error {
    return fmt.Errorf("not implemented")
}

func (c *HTTPNodeClient) GetKey(ctx context.Context, key string) ([]byte, int64, error) {
    return nil, 0, fmt.Errorf("not implemented")
}

func (c *HTTPNodeClient) DeleteKey(ctx context.Context, key string, version int64) error {
    return fmt.Errorf("not implemented")
}

func (c *HTTPNodeClient) TransferKeys(ctx context.Context, start, end *big.Int) (map[string][]byte, error) {
    return nil, fmt.Errorf("not implemented")
}