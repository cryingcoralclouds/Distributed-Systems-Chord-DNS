package dns

import (
	"chord_dns/chord"
	// "fmt"
)

func RouteQuery(node *chord.Node) *chord.Node {
	if node.ID.Cmp(key) == 0 || (node.Predecessor != nil && node.Predecessor.ID.Cmp(key) < 0 && node.ID.Cmp(key) >= 0) {
		return node // This node is responsible for the key
	}

	M := len(node.FingerTable)
	for i := M - 1; i >= 0; i-- {
		if node.FingerTable[i] != nil && node.FingerTable[i].ID.Cmp(node.ID) != 0 && node.FingerTable[i].ID.Cmp(key) <= 0 {
			return RouteQuery(node.FingerTable[i]) // Forward to closest preceding node
		}
	}
	return RouteQuery(node.Successor) // Forward to the successor if no closer node in the finger table
}

func RouteQueryWithFailureHandling(node *chord.Node) *chord.Node {
	target := RouteQuery(node)
	for i := 0; target == nil && i < len(node.Successors); i++ {
		target = RouteQuery(node.Successors[i]) // Try the next successor
	}
	return target
}
