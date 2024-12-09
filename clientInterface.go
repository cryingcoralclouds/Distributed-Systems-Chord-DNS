package main

import (
	"chord_dns/chord"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Mock introducer nodes
// var introducerNodes = nodes []ChordNode

// Client configuration
var (
	concurrentRequests  = 3 // Can be made configurable
	timeout             = 10 * time.Second
	maxRetries          = 3
	interval            = 5 * time.Second
	automatedDomainList = []string{"google.com", "netflix.com", "facebook.com", "random.org", "#cdf", "googletagmanager.com"} // Example domains for testing
)

// check whether the domain name is legal
func isLegalDomain(domain string) bool {
	// Check if domain is empty
	if domain == "" {
		return false
	}

	// Check if domain starts or ends with dot or hyphen
	if strings.HasPrefix(domain, ".") || strings.HasPrefix(domain, "-") ||
		strings.HasSuffix(domain, ".") || strings.HasSuffix(domain, "-") {
		return false
	}

	// Check for at least one dot
	if !strings.Contains(domain, ".") {
		return false
	}

	// Check each character
	for _, char := range domain {
		// Allow letters, numbers, dots, and hyphens
		if !unicode.IsLetter(char) && !unicode.IsNumber(char) &&
			char != '.' && char != '-' {
			return false
		}
	}

	return true
}

// resolveDomain handles domain resolution with retries
func resolveDomain(nodes []ChordNode, domain string, wg *sync.WaitGroup, results chan<- string, logFile *os.File) {
	defer wg.Done()
	var elapsed time.Duration
	var success bool
	var legal bool

	hashedDomain := chord.HashKey(domain).String()

	for i := 0; i < maxRetries; i++ {
		// TO DO: question should the time taken to check the validity of the domain be counted in elapsed time?
		node := nodes[rand.Intn(len(nodes))] // Choose a random node
		start := time.Now()

		// check for legal domain
		legal = isLegalDomain(domain)
		if !legal {
			break
		}

		// print node
		fmt.Println(node)

		ip, err := node.node.Get(hashedDomain)
		fmt.Println(ip)
		fmt.Println(err)
		elapsed = time.Since(start)

		if err == nil {
			logEntry := fmt.Sprintf("Node: %v, Domain: %s, Resolved IP: %s, Elapsed Time: %v\n", node, domain, ip, elapsed)
			results <- logEntry
			logFile.WriteString(logEntry)
			success = true
			break
		} else {
			logEntry := fmt.Sprintf("Domain: %s, Attempt: %d, Timeout\n", domain, i+1)
			results <- logEntry
			logFile.WriteString(logEntry)
			time.Sleep(timeout) // Retry delay
		}
	}

	if !legal {
		logEntry := fmt.Sprintf("Domain: %s, Resolution failed, illegal domain name\n", domain)
		results <- logEntry
		logFile.WriteString(logEntry)
	}

	if !success {
		logEntry := fmt.Sprintf("Domain: %s, Resolution failed after %d attempts\n", domain, maxRetries)
		results <- logEntry
		logFile.WriteString(logEntry)
	}
}

// automatedTesting performs automated testing on a list of domains
func automatedTesting(nodes []ChordNode) {
	logFile, err := os.OpenFile("./results/automated.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
	}
	defer logFile.Close() //defer executes when all the subrouties below/above have finished

	for {
		var wg sync.WaitGroup
		results := make(chan string, len(automatedDomainList)*maxRetries)
		// randNode := nodes[rand.Intn(len(nodes))] // Choose a random node
		// mockNode := &Node{Name: randNode}

		for i, domain := range automatedDomainList {
			if i%concurrentRequests == 0 {
				wg.Wait()
			}
			wg.Add(1)
			go resolveDomain(nodes, domain, &wg, results, logFile)
		}

		wg.Wait()
		close(results)
		for result := range results {
			fmt.Println(result)
		}

		time.Sleep(interval)
	}

}

// manualResolution allows manual testing and returns to main menu after exit
func manualResolution(nodes []ChordNode) {
	logFile, err := os.OpenFile("./results/manual.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}
	defer logFile.Close()

	var domain string
	fmt.Println("Enter domain names to resolve (type 'exit' to return to main menu):")
	for {
		fmt.Print("Domain: ")
		fmt.Scanln(&domain)
		if domain == "exit" {
			runClientInterface(nodes) // Return to main menu
			return
		}

		var wg sync.WaitGroup
		results := make(chan string, 1)

		wg.Add(1)
		go resolveDomain(nodes, domain, &wg, results, logFile)

		wg.Wait()
		close(results)
		for result := range results {
			fmt.Println(result)
		}
	}
}

func runClientInterface(nodes []ChordNode) {
	rand.Seed(time.Now().UnixNano())
	for {
		fmt.Println("\nMain Menu:")
		fmt.Println("1. Manual Domain Name Resolution")
		fmt.Println("2. Automated Testing")
		fmt.Println("3. Exit Program")

		var choice int
		fmt.Print("Enter your choice: ")
		fmt.Scanln(&choice)

		switch choice {
		case 1:
			manualResolution(nodes)
			return // Return after manual resolution completes
		case 2:
			automatedTesting(nodes)
			return // Return after automated testing completes
		case 3:
			fmt.Println("Exiting program.")
			os.Exit(0)
		default:
			fmt.Println("Invalid choice. Please try again.")
		}
	}
}
