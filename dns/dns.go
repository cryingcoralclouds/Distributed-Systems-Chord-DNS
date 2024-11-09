package dns

import (
	"crypto/md5"
	"encoding/json"
	"math/big"
	"math/rand"
	"os"
)

func HashKey(input string) *big.Int {
	// Generate a seed using MD5 hash of the input string for deterministic random behaviour
	seed := md5.Sum([]byte(input))

	// Create a new random source with the seed
	r := rand.New(rand.NewSource(int64(seed[0]) | int64(seed[1])<<8 | int64(seed[2])<<16 | int64(seed[3])<<24))

	maxInt := big.NewInt(1024) // Key space of 2^12

	// Generate a pseudo-random number in the reduced key space
	randomBigInt := new(big.Int).Rand(r, maxInt)

	return randomBigInt
}

// ProcessDNSData reads the input JSON file, hashes the domain names, and writes the output JSON file.
func ProcessDNSData(inputFile, outputFile string) error {
	// Read the input JSON file
	file, err := os.ReadFile(inputFile)
	if err != nil {
		return err
	}

	// Parse the JSON data into a map
	var dnsData map[string]string
	err = json.Unmarshal(file, &dnsData)
	if err != nil {
		return err
	}

	// Create a new map to store hashed domain names
	hashedData := make(map[string]string)

	// Hash each domain name
	for domain, ip := range dnsData {
		hashedKey := HashKey(domain)
		hashedData[hashedKey.String()] = ip
	}

	// Convert the hashed data to JSON
	hashedJSON, err := json.MarshalIndent(hashedData, "", "  ")
	if err != nil {
		return err
	}

	// Write the hashed data to the output JSON file
	err = os.WriteFile(outputFile, hashedJSON, 0644)
	if err != nil {
		return err
	}

	return nil
}
