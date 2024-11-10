package chord

type DNSResponse struct {
    IP      string `json:"ip,omitempty"`
    Found   bool   `json:"found"`
    Version int64  `json:"version"`
}

// NodeMsg represents messages between nodes
type NodeMsg struct {
	Type    string
	Payload interface{}
	Reply   chan interface{}
}