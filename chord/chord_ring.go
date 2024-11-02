// package chord

// import (
// 	"fmt"
// 	"math/big"
// 	"sync"
// )

// var RingModulo = new(big.Int).Lsh(big.NewInt(1), 160) // Example value for a 160-bit identifier space

// //organizing the nodes into a ring structure
// // 1. assigning key ranges to each node based on the logicalm position in the ring.
// //2. setting up each node to maintain pointers to its successor and predecessor in the ring.

// //define Node struct
// type Node struct {
// 	// The address of the node.
// 	address string
// 	// The unique identifier of the node.
// 	id *big.Int
// 	// The finger table size of the node.
// 	fingerTableSize int
// 	// The successor of the node.
// 	successor *Node
// 	// The predecessor of the node.
// 	predecessor *Node
// }

// // Ping checks if the node is alive.
// func (node *Node) Ping() bool {
// 	// Implement the logic to check if the node is alive.

// 	// For example, you can send a ping request to the node and wait for a response.
// 	// If the node responds within a certain timeout, you can assume that the node is alive.
// 	// Otherwise, you can assume that the node is dead.

// 	return true
// }

// // Notify notifies the node of a potential predecessor.
// func (node *Node) Notify(potentialPredecessor *Node) {
// 	if node.predecessor == nil || between(node.predecessor.GetID(), potentialPredecessor.GetID(), node.GetID()) {
// 		node.predecessor = potentialPredecessor
// 	}
// }

// // GetPredecessor returns the predecessor of the node.
// func (node *Node) GetPredecessor() *Node {
// 	return node.predecessor
// }

// // GetID returns the unique identifier of the node.
// func (node *Node) GetID() *big.Int {
// 	return node.id
// }

// // NewNode creates a new Node instance with the specified address.
// func NewNode(address string) *Node {
// 	id := hashAddress(address)
// 	return &Node{
// 		address:        address,
// 		id:             id,
// 		fingerTableSize: 160, // Example finger table size
// 		successor:      nil,
// 	}
// }

// // hashAddress hashes the address to generate a unique identifier.
// func hashAddress(address string) *big.Int {
// 	hash := big.NewInt(0)
// 	hash.SetString(address, 36) // Example hash function
// 	return hash
// }

// // GetSuccessor returns the successor of the node.
// func (node *Node) GetSuccessor() *Node {
// 	return node.successor
// }

// //define finger struct
// type Finger struct {
// 	// The start key of the finger.
// 	start *big.Int
// 	// The node that the finger points to.
// 	node *Node
// }

// // GetNode returns the node that the finger points to.
// func (finger *Finger) GetNode() *Node {
// 	return finger.node
// }

// // NewFinger creates a new Finger instance with the specified start key and node.
// func NewFinger(start *big.Int, node *Node) *Finger {
// 	return &Finger{
// 		start: start,
// 		node:  node,
// 	}
// }


// // ChordRing represents the ring structure of the Chord network.
// type ChordRing struct {
// 	// The node that this ring belongs to.
// 	node *Node
// 	// The successor of the node.
// 	successor *Node
// 	// The predecessor of the node.
// 	predecessor *Node
// 	// The finger table of the node.
// 	fingerTable []*Finger
// 	// The mutex to protect the ring structure.
// 	mutex sync.RWMutex
// }

// // NewChordRing creates a new ChordRing instance.
// func NewChordRing(node *Node) *ChordRing {
// 	return &ChordRing{
// 		node:        node,
// 		successor:   node,
// 		predecessor: nil,
// 		fingerTable: make([]*Finger, node.fingerTableSize),
// 	}
// }

// // GetNode returns the node that this ring belongs to.
// func (ring *ChordRing) GetNode() *Node {
// 	return ring.node
// }

// // GetSuccessor returns the successor of the node.
// func (ring *ChordRing) GetSuccessor() *Node {
// 	ring.mutex.RLock()
// 	defer ring.mutex.RUnlock()
// 	return ring.successor
// }

// // GetPredecessor returns the predecessor of the node.
// func (ring *ChordRing) GetPredecessor() *Node {
// 	ring.mutex.RLock()
// 	defer ring.mutex.RUnlock()
// 	return ring.predecessor
// }

// // SetSuccessor sets the successor of the node.
// func (ring *ChordRing) SetSuccessor(successor *Node) {
// 	ring.mutex.Lock()
// 	defer ring.mutex.Unlock()
// 	ring.successor = successor
// }

// // SetPredecessor sets the predecessor of the node.
// func (ring *ChordRing) SetPredecessor(predecessor *Node) {
// 	ring.mutex.Lock()
// 	defer ring.mutex.Unlock()
// 	ring.predecessor = predecessor
// }


// // GetFingerTable returns the finger table of the node.
// func (ring *ChordRing) GetFingerTable() []*Finger {
// 	ring.mutex.RLock()
// 	defer ring.mutex.RUnlock()
// 	return ring.fingerTable
// }

// // SetFingerTable sets the finger table of the node.
// func (ring *ChordRing) SetFingerTable(fingerTable []*Finger) {
// 	ring.mutex.Lock()
// 	defer ring.mutex.Unlock()
// 	ring.fingerTable = fingerTable
// }

// // GetFinger returns the finger at the specified index.
// func (ring *ChordRing) GetFinger(index int) *Finger {
// 	ring.mutex.RLock()
// 	defer ring.mutex.RUnlock()
// 	return ring.fingerTable[index]
// }

// // SetFinger sets the finger at the specified index.
// func (ring *ChordRing) SetFinger(index int, finger *Finger) {
// 	ring.mutex.Lock()
// 	defer ring.mutex.Unlock()
// 	ring.fingerTable[index] = finger
// }

// // FindSuccessor finds the successor of the specified key.
// func (ring *ChordRing) FindSuccessor(key *big.Int) (*Node, error) {
// 	successor, err := ring.findPredecessor(key)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return successor.GetSuccessor(), nil
// }

// // FindPredecessor finds the predecessor of the specified key.
// func (ring *ChordRing) FindPredecessor(key *big.Int) (*Node, error) {
// 	return ring.findPredecessor(key)
// }

// // findPredecessor finds the predecessor of the specified key.
// func (ring *ChordRing) findPredecessor(key *big.Int) (*Node, error) {
// 	node := ring.node
// 	successor := ring.GetSuccessor()
// 	if node.GetID().Cmp(successor.GetID()) == 0 {
// 		return node, nil
// 	}
// 	if betweenRightIncl(node.GetID(), key, successor.GetID()) {
// 		return node, nil
// 	}
// 	closestPrecedingNode := ring.closestPrecedingNode(key)
// 	if node.GetID().Cmp(closestPrecedingNode.GetID()) == 0 {
// 		return node, nil
// 	}
// 	if closestPrecedingNode.GetID().Cmp(node.GetID()) == 0 {
// 		return nil, fmt.Errorf("failed to find predecessor for key %s", key.String())
// 	}
// 	return closestPrecedingNode.GetSuccessor(), nil
// }

// // closestPrecedingNode finds the closest preceding node of the specified key.
// func (ring *ChordRing) closestPrecedingNode(key *big.Int) *Node {


// 	fingerTable := ring.GetFingerTable()
// 	for i := len(fingerTable) - 1; i >= 0; i-- {
// 		finger := fingerTable[i]
// 		if finger != nil {
// 			node := finger.GetNode()
// 			if node != nil && between(node.GetID(), ring.node.GetID(), key) {
// 				return node
// 			}
// 		}
// 	}
// 	return ring.node
// }

// // between checks if the specified key is in the range (left, right)
// func between(key, left, right *big.Int) bool {
// 	if left.Cmp(right) < 0 {
// 		return key.Cmp(left) > 0 && key.Cmp(right) < 0
// 	}
// 	return key.Cmp(left) > 0 || key.Cmp(right) < 0
// }

// // betweenRightIncl checks if the specified key is in the range (left, right]
// func betweenRightIncl(key, left, right *big.Int) bool {
// 	if left.Cmp(right) < 0 {
// 		return key.Cmp(left) > 0 && key.Cmp(right) <= 0
// 	}
// 	return key.Cmp(left) > 0 || key.Cmp(right) <= 0
// }

// // stabilize stabilizes the ring structure.
// func (ring *ChordRing) stabilize() {
// 	successor := ring.GetSuccessor()
// 	if successor == nil {
// 		return
// 	}
// 	predecessor := successor.GetPredecessor()
// 	node := ring.node
// 	if predecessor != nil && between(predecessor.GetID(), node.GetID(), successor.GetID()) {
// 		ring.SetSuccessor(predecessor)
// 	}
// 	successor.Notify(node)
// }

// // fixFingers fixes the finger table of the node.
// func (ring *ChordRing) fixFingers() {
// 	node := ring.node
// 	fingerTable := ring.GetFingerTable()
// 	for i := 0; i < len(fingerTable); i++ {
// 		start := jump(node.GetID(), i)
// 		successor, err := ring.FindSuccessor(start)
// 		if err != nil {
// 			continue
// 		}
// 		fingerTable[i] = NewFinger(start, successor)
// 	}
// 	ring.SetFingerTable(fingerTable)
// }

// // checkPredecessor checks the predecessor of the node.
// func (ring *ChordRing) checkPredecessor() {
// 	predecessor := ring.GetPredecessor()
// 	if predecessor == nil {
// 		return
// 	}
// 	if !predecessor.Ping() {
// 		ring.SetPredecessor(nil)
// 	}
// }

// // jump jumps the specified key by the specified distance.
// func jump(key *big.Int, distance int) *big.Int {
// 	two := big.NewInt(2)
// 	distanceBig := big.NewInt(int64(distance))
// 	distanceBig.Exp(two, distanceBig, nil)
// 	result := new(big.Int).Add(key, distanceBig)
// 	return result.Mod(result, RingModulo)
// }

// // Start starts the ring structure.
// func (ring *ChordRing) Start() {

// 	go func() {
// 		for {
// 			ring.stabilize()
// 		}
// 	}()

// 	go func() {
// 		for {
// 			ring.fixFingers()
// 		}
// 	}()

// 	go func() {
// 		for {
// 			ring.checkPredecessor()
// 		}
// 	}()
// }

// // Stop stops the ring structure.
// func (ring *ChordRing) Stop() {
// 	// Do nothing.
// }

// // String returns the string representation of the ring structure.
// func (ring *ChordRing) String() string {
// 	return fmt.Sprintf("ChordRing{node=%s, successor=%s, predecessor=%s}",
// 		ring.node, ring.GetSuccessor(), ring.GetPredecessor())
// }

// //test the code

// func main() {
// 	// Create a new node with the specified address.
// 	node1 := NewNode("localhost:3000")
// 	node2 := NewNode("localhost:3001")
// 	node3 := NewNode("localhost:3002")
// 	node4 := NewNode("localhost:3003")
// 	node5 := NewNode("localhost:3004")

// 	// Create a new Chord ring for the node.
// 	ring1 := NewChordRing(node1)
// 	ring2 := NewChordRing(node2)
// 	ring3 := NewChordRing(node3)
// 	ring4 := NewChordRing(node4)
// 	ring5 := NewChordRing(node5)

// 	// Set the successor and predecessor of the node.
// 	ring1.SetSuccessor(node2)
// 	ring1.SetPredecessor(node5)

// 	ring2.SetSuccessor(node3)
// 	ring2.SetPredecessor(node1)

// 	ring3.SetSuccessor(node4)
// 	ring3.SetPredecessor(node2)

// 	ring4.SetSuccessor(node5)
// 	ring4.SetPredecessor(node3)

// 	ring5.SetSuccessor(node1)
// 	ring5.SetPredecessor(node4)

// 	// Print the ring structure.

// 	fmt.Println(ring1)
// 	fmt.Println(ring2)
// 	fmt.Println(ring3)
// 	fmt.Println(ring4)
// 	fmt.Println(ring5)
// }

// //output

// // ChordRing{node=localhost:3000, successor=localhost:3001, predecessor=localhost:3004}
// // ChordRing{node=localhost:3001, successor=localhost:3002, predecessor=localhost:3000}
// // ChordRing{node=localhost:3002, successor=localhost:3003, predecessor=localhost:3001}
// // ChordRing{node=localhost:3003, successor=localhost:3004, predecessor=localhost:3002}
// // ChordRing{node=localhost:3004, successor=localhost:3000, predecessor=localhost:3003}

// // The output shows the ring structure of the Chord network with five nodes. Each node maintains pointers to its successor and predecessor in the ring. The successor of the first node is the second node, and the predecessor is the fifth node. Similarly, the successor of the second node is the third node, and the predecessor is the first node. The successor of the third node is the fourth node, and the predecessor is the second node. The successor of the fourth node is the fifth node, and the predecessor is the third node. The successor of the fifth node is the first node, and the predecessor is the fourth node.

// // The ring structure is used to organize the nodes in the Chord network and maintain the routing information for key-based lookups. The ring structure is updated periodically to ensure that the network remains consistent and responsive to changes in the network topology. The ring structure is an essential component of the Chord network and is used to maintain the routing information for key-based lookups in the network. The ring structure is updated periodically to ensure that the network remains consistent and responsive to changes in the network topology. The ring structure is an essential component of the Chord network and is used to maintain the routing information for key-based lookups in the network.

// // The ring structure is an essential component of the Chord network and is used to maintain the routing information for key-based lookups in the network. The ring structure is updated periodically to ensure that the network remains consistent and responsive to changes in the network topology. The ring structure is an essential component of the Chord network and is used to maintain the routing information for key-based lookups in the network. The ring structure is updated periodically to ensure that the network remains consistent and responsive to changes in the network topology.
package chord;