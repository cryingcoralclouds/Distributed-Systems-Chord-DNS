package chord

import (
	"context"
	"log"
	"sync"
	"time"
)

// HeartbeatManager periodically checks the health of successors and predecessors.
type HeartbeatManager struct {
	Node      *Node
	Interval  time.Duration
	StopChan  chan struct{}
	WaitGroup *sync.WaitGroup
}

// NewHeartbeatManager initializes a new HeartbeatManager for the given node.
func NewHeartbeatManager(node *Node, interval time.Duration) *HeartbeatManager {
	return &HeartbeatManager{
		Node:      node,
		Interval:  interval,
		StopChan:  make(chan struct{}),
		WaitGroup: &sync.WaitGroup{},
	}
}

// Start begins the periodic heartbeat checks.
func (hm *HeartbeatManager) Start() {
	hm.WaitGroup.Add(1)
	go func() {
		defer hm.WaitGroup.Done()
		for {
			select {
			case <-hm.StopChan:
				log.Printf("HeartbeatManager stopped for Node %s", hm.Node.ID.String())
				return
			default:
				hm.checkNodeHealth()
				hm.checkSuccessors()
				hm.checkPredecessor()
				time.Sleep(hm.Interval)
			}
		}
	}()
}

// Stop halts the heartbeat checks.
func (hm *HeartbeatManager) Stop() {
	close(hm.StopChan)
	hm.WaitGroup.Wait()
}

// checkSuccessors pings the successors to ensure they are alive.
func (hm *HeartbeatManager) checkSuccessors() {
	for i, successor := range hm.Node.Successors {
		if successor == nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := successor.Client.Ping(ctx)
		if err != nil {
			log.Printf("[Heartbeat] Successor %d (%s) is unresponsive: %v", i, successor.ID.String(), err)
			if i == 0 {
				hm.Node.fixSuccessor() // Attempt to update the immediate successor
			}
		}

		if !hm.isNodeAlive(successor) {
			log.Printf("[Heartbeat] Successor %d (%s) is unresponsive", i, successor.ID.String())
			if i == 0 {
				hm.Node.fixSuccessor() // Attempt to update the immediate successor
			}
		}
	}
}

// checkPredecessor pings the predecessor to ensure it is alive.
func (hm *HeartbeatManager) checkPredecessor() {
	if hm.Node.Predecessor == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := hm.Node.Predecessor.Client.Ping(ctx)
	if err != nil {
		log.Printf("[Heartbeat] Predecessor (%s) is unresponsive: %v", hm.Node.Predecessor.ID.String(), err)
		hm.Node.Predecessor = nil // Remove unresponsive predecessor
	}

	if !hm.isNodeAlive(hm.Node.Predecessor) {
		log.Printf("[Heartbeat] Predecessor (%s) is unresponsive", hm.Node.Predecessor.ID.String())
		hm.Node.Predecessor = nil // Remove unresponsive predecessor
	}
}

// checkNodeHealth ensures the local node is alive.
func (hm *HeartbeatManager) checkNodeHealth() {
	if !hm.Node.IsAlive {
		log.Printf("[Heartbeat] Local Node %s is marked as not alive.", hm.Node.ID.String())
		return
	}
}

// isNodeAlive checks if a node is alive using the Ping method and the IsAlive property.
func (hm *HeartbeatManager) isNodeAlive(node *RemoteNode) bool {
	if !node.IsAlive {
		log.Printf("[Heartbeat] Node %s failed heartbeat check.", node.ID.String())
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := node.Client.Ping(ctx)
	if err != nil {
		log.Printf("[Heartbeat] Node %s failed Ping check: %v", node.ID.String(), err)
		node.IsAlive = false // Mark node as not alive
		return false
	}

	node.IsAlive = true // Confirm node is alive
	return true
}

// fixSuccessor updates the immediate successor if it is unresponsive.
func (n *Node) fixSuccessor() {
	if len(n.Successors) < 2 {
		return
	}

	n.Successors = n.Successors[1:] // Shift the successor list to the left
	log.Printf("[Heartbeat] Updated immediate successor for Node %s", n.ID.String())

	if len(n.Successors) < 1 {
		log.Printf("[Heartbeat] Node %s has no more successors", n.ID.String())
	}
}
