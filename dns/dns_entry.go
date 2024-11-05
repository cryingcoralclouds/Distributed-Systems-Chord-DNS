package dns

import (
	"bufio"
	"chord_dns/chord"
	"crypto/sha1"
	"fmt"
	"math/big"
	"os"
	"strings"
)

// DNSEntry represents a domain name and its associated IP address.
type DNSEntry struct {
	Domain string
	IP     string
}

// hashDomain takes a domain string and returns a big integer hash.
func hashDomain(domain string) *big.Int {
	h := sha1.New()
	h.Write([]byte(domain))
	return new(big.Int).SetBytes(h.Sum(nil))
}

// LoadDNSFromFile reads a text file containing domain-IP pairs and returns a slice of DNSEntry structs.
func LoadDNSFromFile(filePath string) ([]DNSEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var entries []DNSEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			fmt.Printf("skipping malformed line: %s\n", line)
			continue
		}

		domain := parts[0]
		ip := parts[1]

		entry := DNSEntry{Domain: domain, IP: ip}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return entries, nil
}

// PopulateDHT populates the DHT by distributing DNS entries across the ring.
func PopulateDHT(node *chord.Node, entries []DNSEntry) {
	for _, entry := range entries {
		// Logic to distribute DNS entries across the ring by hashing the domain name handled by seperate fn
		StoreDNSEntry(node, entry)
	}
}
