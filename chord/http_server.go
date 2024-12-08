package chord

/*
HTTPNodeServer:
* Hosts an HTTP server for a node.
* Configures routes (currently just /ping) to allow other nodes to interact with it.
* Starts listening on the specified address and responds to HTTP requests.
*/

import (
	"encoding/json"
	// "io"
	"log"
	"math/big"
	"net/http"
	// "time"
)

type HTTPNodeServer struct {
	node *Node
}

func NewHTTPNodeServer(node *Node) *HTTPNodeServer {
	return &HTTPNodeServer{node: node}
}

// Configure the basic HTTP routes for the node
func (s *HTTPNodeServer) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Expose endpoints
	mux.HandleFunc("/ping", s.handlePing)
	mux.HandleFunc("/successor/", s.handleFindSuccessor)
	mux.HandleFunc("/successors", s.handleGetSuccessors)
	mux.HandleFunc("/predecessor", s.handleGetPredecessor)
	mux.HandleFunc("/notify", s.handleNotify)
	mux.HandleFunc("/store/", s.handleStoreKey)
	mux.HandleFunc("/key/", s.handleGetKey)
    mux.HandleFunc("/store-replica/", s.handleStoreReplica)
    mux.HandleFunc("/transfer-keys", s.handleTransferKeys)

	return mux
}

// Handle Ping requests from client
func (s *HTTPNodeServer) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := struct {
		Status string `json:"status"`
		NodeID string `json:"nodeId"`
	}{
		Status: "alive",
		NodeID: s.node.ID.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Start server
func (s *HTTPNodeServer) Start(address string) error {
	server := &http.Server{
		Addr:    address,
		Handler: s.SetupRoutes(),
	}

	log.Printf("Starting Chord node HTTP server on %s", address)
	return server.ListenAndServe()
}

func (s *HTTPNodeServer) handleFindSuccessor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path
	idStr := r.URL.Path[len("/successor/"):]
	id, ok := new(big.Int).SetString(idStr, 10)
	if !ok {
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	successor := s.node.FindResponsibleNode(id)
	if successor == nil {
		http.Error(w, "No successor found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(successor); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (s *HTTPNodeServer) handleGetSuccessors(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    successors := s.node.GetSuccessors()
    if successors == nil {
        successors = []*RemoteNode{} // Return empty array instead of null
    }

    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(successors); err != nil {
        log.Printf("Error encoding successors: %v", err)
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }
}

func (s *HTTPNodeServer) handleGetPredecessor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.node.Predecessor == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.node.Predecessor)
}

func (s *HTTPNodeServer) handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var node RemoteNode
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	shouldUpdate := false
	if s.node.Predecessor == nil {
		shouldUpdate = true
	} else if s.node.Predecessor.ID.Cmp(s.node.ID) == 0 {
		// If we're our own predecessor, accept the new node
		shouldUpdate = true
	} else {
		// Check if the new node is between our current predecessor and us
		startID := s.node.Predecessor.ID
		endID := s.node.ID

		// If start > end, we've wrapped around the ring
		if startID.Cmp(endID) > 0 {
			// Accept if node is greater than start OR less than end
			shouldUpdate = node.ID.Cmp(startID) > 0 || node.ID.Cmp(endID) < 0
		} else {
			// Normal case: node should be between start and end
			shouldUpdate = node.ID.Cmp(startID) > 0 && node.ID.Cmp(endID) < 0
		}
	}

	if shouldUpdate {
		s.node.Predecessor = &node
	}

	w.WriteHeader(http.StatusOK)
}

func (s *HTTPNodeServer) handleStoreKey(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    key := r.URL.Path[len("/store/"):]
    if key == "" {
        http.Error(w, "Key not provided", http.StatusBadRequest)
        return
    }

    var metadata KeyMetadata
    if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil {
        http.Error(w, "Failed to decode request body", http.StatusBadRequest)
        return
    }

    // Find responsible node
    hash := new(big.Int)
    hash.SetString(key, 10)
    responsible := s.node.FindResponsibleNode(hash)

    if responsible.ID.Cmp(s.node.ID) == 0 {
        // We are responsible, store as primary
        metadata.IsPrimary = true
        metadata.PrimaryNode = &RemoteNode{
            ID:      s.node.ID,
            Address: s.node.Address,
            Client:  s.node.Client,
        }
        s.node.DHT[key] = metadata
        
        // Replicate to successors
        successCount := 0
        
        // Log successor list
        for i, succ := range s.node.Successors {
            if succ != nil {
                log.Printf("  Successor[%d]: %s", i, succ.ID)
            } else {
                log.Printf("  Successor[%d]: nil", i)
            }
        }

        for i := 0; i < len(s.node.Successors) && successCount < ReplicationFactor-1; i++ {
            if s.node.Successors[i] != nil && s.node.Successors[i].ID.Cmp(s.node.ID) != 0 {
                
                replicaData := KeyMetadata{
                    Value:      metadata.Value,
                    Version:    metadata.Version,
                    IsPrimary:  false,
                    PrimaryNode: metadata.PrimaryNode,
                }
                
                ctx := r.Context()
                err := s.node.Successors[i].Client.StoreReplica(ctx, key, replicaData)
                if err == nil {
                    successCount++
                    s.node.ReplicationStatus[key] = append(s.node.ReplicationStatus[key], s.node.Successors[i])
                } else {
                    log.Printf("[handleStoreKey] Failed to replicate to successor %s: %v", 
                        s.node.Successors[i].ID, err)
                }
            }
        }
        
        w.WriteHeader(http.StatusOK)
        return
    }

    // Forward to responsible node
    ctx := r.Context()
    if err := responsible.Client.StoreKey(ctx, key, metadata); err != nil {
        http.Error(w, "Failed to forward store request", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}

func (s *HTTPNodeServer) handleGetKey(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    key := r.URL.Path[len("/key/"):]
    if key == "" {
        http.Error(w, "Key not provided", http.StatusBadRequest)
        return
    }

    hash := new(big.Int)
    hash.SetString(key, 10)
    responsible := s.node.FindResponsibleNode(hash)

    if responsible.ID.Cmp(s.node.ID) == 0 {
        // We are responsible, look up in our DHT
        metadata, exists := s.node.DHT[key]
        if !exists {
            w.WriteHeader(http.StatusNotFound)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(metadata)
        return
    }

    // Forward to responsible node
    ctx := r.Context()
    value, version, err := responsible.Client.GetKey(ctx, key)
    if err != nil {
        if err == ErrKeyNotFound {
            w.WriteHeader(http.StatusNotFound)
            return
        }
        http.Error(w, "Failed to forward request", http.StatusInternalServerError)
        return
    }

    // Create metadata from the received values
    metadata := KeyMetadata{
        Value:    value,
        Version:  version,
        IsPrimary: false,  // Since we received it from another node
        PrimaryNode: responsible,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(metadata)
}

func (s *HTTPNodeServer) handleStoreReplica(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    key := r.URL.Path[len("/store-replica/"):]
    if key == "" {
        http.Error(w, "Key not provided", http.StatusBadRequest)
        return
    }

    var metadata KeyMetadata
    if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil {
        http.Error(w, "Failed to decode request body", http.StatusBadRequest)
        return
    }

    // Ensure we're storing as replica
    metadata.IsPrimary = false
    s.node.DHT[key] = metadata
    w.WriteHeader(http.StatusOK)
}

func (s *HTTPNodeServer) handleTransferKeys(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Decode request body
    var req struct {
        Start string `json:"start"`
        End   string `json:"end"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Convert string hashes to big.Int
    start := new(big.Int)
    end := new(big.Int)
    if _, ok := start.SetString(req.Start, 10); !ok {
        http.Error(w, "Invalid start hash", http.StatusBadRequest)
        return
    }
    if _, ok := end.SetString(req.End, 10); !ok {
        http.Error(w, "Invalid end hash", http.StatusBadRequest)
        return
    }

    // Get keys in range
    keys, err := s.node.TransferKeys(start, end)
    if err != nil {
        http.Error(w, "Failed to transfer keys", http.StatusInternalServerError)
        return
    }

    // Prepare response
    response := struct {
        Keys map[string][]byte `json:"keys"`
    }{
        Keys: keys,
    }

    // Send response
    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(response); err != nil {
        log.Printf("Error encoding response: %v", err)
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }
}
