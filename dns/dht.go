package dns

import (
	"chord_dns/chord"
	"fmt"
)

func StoreDNSEntry(node *chord.Node, entry DNSEntry) {
	key := hashDomain(entry.Domain)
	responsibleNode := node.RouteQueryWithFailureHandling(key)
	if responsibleNode != nil {
		responsibleNode.Store(entry) // Store DNS entry in responsible node
	}
}

func (node *chord.Node) Store(entry DNSEntry) {
	// Store the DNS entry in the node's keys map
	hashDomain := hashDomain(entry.Domain)
	node.Keys[entry.Domain] = entry.IP
	fmt.Printf("Stored domain %s with IP %s on node %s\n", entry.Domain, entry.IP, node.Address)
}
func RetrieveDNSEntry(node *chord.Node, domain string) (string, bool) {
	// Logic to retrieve a DNS entry by routing to the responsible node
}

func (node *chord.Node) RetrieveDNSEntry(domain string) (string, bool) {
	key := hashDomain(domain)
	responsibleNode := node.RouteQueryWithFailureHandling(key)
	if responsibleNode != nil {
		return responsibleNode.Retrieve(domain)
	}
	return "", false
}

func (node *chord.Node) Retrieve(domain string) (string, bool) {
	node.Mutex.RLock()
	defer node.Mutex.RUnlock()

	ip, exists := node.DNSData[domain]
	if exists {
		fmt.Printf("Retrieved IP %s for domain %s on node %s\n", ip, domain, node.Address)
	}
	return ip, exists
}
