package chord

/*
HTTPNodeServer:
* Hosts an HTTP server for a node.
* Configures routes (currently just /ping) to allow other nodes to interact with it.
* Starts listening on the specified address and responds to HTTP requests.
*/

import (
	"encoding/json"
	"io"
	"log"
	"math/big"
	"net/http"
	"time"
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
    mux.HandleFunc("/predecessor", s.handleGetPredecessor)
	mux.HandleFunc("/notify", s.handleNotify)
	mux.HandleFunc("/store/", s.handleStoreKey)
	mux.HandleFunc("/key/", s.handleGetKey)

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

    successor := s.node.findSuccessorInternal(id)
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

	// Parse the key from the URL path
	key := r.URL.Path[len("/store/"):]
	if key == "" {
		http.Error(w, "Key not provided", http.StatusBadRequest)
		return
	}

	// Read the value from request body
	value, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Store the key-value pair
	s.node.DHT[key] = value
	s.node.Versions[key] = time.Now().UnixNano()

	w.WriteHeader(http.StatusOK)
}

func (s *HTTPNodeServer) handleGetKey(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Parse the key from the URL path
    key := r.URL.Path[len("/key/"):]
    if key == "" {
        http.Error(w, "Key not provided", http.StatusBadRequest)
        return
    }

    // Get the value and version
    value, exists := s.node.DHT[key]
    if !exists {
        http.Error(w, "Key not found", http.StatusNotFound)
        return
    }
    
    version := s.node.Versions[key]

    // Create response
    response := struct {
        Value   []byte `json:"value"`
        Version int64  `json:"version"`
    }{
        Value:   value,
        Version: version,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
