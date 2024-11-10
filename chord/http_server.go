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

	// Read value from body
	value, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Find responsible node
	hash := new(big.Int)
	hash.SetString(key, 10)
	responsible := s.node.FindResponsibleNode(hash)

	if responsible.ID.Cmp(s.node.ID) == 0 {
		// We are responsible, store locally
		s.node.DHT[key] = value
		w.WriteHeader(http.StatusOK)
		return
	}

	// We're not responsible - forward to closest preceding node
	closestPred := s.node.GetClosestPrecedingFinger(hash)
	if closestPred == nil {
		http.Error(w, "No valid node found for forwarding", http.StatusInternalServerError)
		return
	}

	// Forward the store request
	ctx := r.Context()
	if err := closestPred.Client.StoreKey(ctx, key, value); err != nil {
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

	hashedDomain := r.URL.Path[len("/key/"):]
	if hashedDomain == "" {
		http.Error(w, "Key not provided", http.StatusBadRequest)
		return
	}

	// Convert hashedDomain to big.Int for Chord operations
	hash := new(big.Int)
	hash.SetString(hashedDomain, 10)

	// Check if this node is responsible
	responsible := s.node.FindResponsibleNode(hash)

	if responsible.ID.Cmp(s.node.ID) == 0 {
		// We are responsible, look up in our DHT
		ip, exists := s.node.DHT[hashedDomain]
		if !exists {
			response := DNSResponse{
				Found:   false,
				Version: 0,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(response)
			return
		}

		version := s.node.Versions[hashedDomain]

		// Return the IP if found
		response := DNSResponse{
			IP:      string(ip),
			Found:   true,
			Version: version,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// We're not responsible - forward to closest preceding node
	closestPred := s.node.GetClosestPrecedingFinger(hash)
	if closestPred == nil {
		http.Error(w, "No valid node found for forwarding", http.StatusInternalServerError)
		return
	}

	// Forward the request
	ctx := r.Context()
	value, version, err := closestPred.Client.GetKey(ctx, hashedDomain)
	if err != nil {
		if err == ErrKeyNotFound {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to forward request", http.StatusInternalServerError)
		return
	}

	// Return the response from the forwarded request
	response := DNSResponse{
		IP:      string(value),
		Found:   true,
		Version: version,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
