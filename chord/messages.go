package chord

// NodeMsg represents messages between nodes
type NodeMsg struct {
	Type    string
	Payload interface{}
	Reply   chan interface{}
}