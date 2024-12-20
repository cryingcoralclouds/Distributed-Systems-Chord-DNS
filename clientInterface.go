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

var (
	// The default concurrency used in standard automated testing
	defaultConcurrentRequests = 3
	concurrentRequests        = defaultConcurrentRequests
	timeout                   = 10 * time.Second
	maxRetries                = 3
	interval                  = 5 * time.Second
	automatedDomainList       = []string{"google.com", "netflix.com", "facebook.com", "random.org", "#cdf", "googletagmanager.com", "google.com", "wikipedia.org", "reddit.com", "facebook.com", "youtube.com", "amazon.com", "twitter.com", "linkedin.com", "instagram.com", "netflix.com", "yahoo.com", "microsoft.com", "apple.com"} // Example domains for testing
)

// Fault injection types
type faultMode int

const (
	noFault faultMode = iota
	intermittentFault
	permanentFault
)

// isLegalDomain checks whether the domain name is valid
func isLegalDomain(domain string) bool {
	if domain == "" {
		return false
	}
	if strings.HasPrefix(domain, ".") || strings.HasPrefix(domain, "-") ||
		strings.HasSuffix(domain, ".") || strings.HasSuffix(domain, "-") {
		return false
	}
	if !strings.Contains(domain, ".") {
		return false
	}

	for _, char := range domain {
		if !unicode.IsLetter(char) && !unicode.IsNumber(char) && char != '.' && char != '-' {
			return false
		}
	}
	return true
}

// faultInjectedGet simulates a node.Get call with possible faults.
func faultInjectedGet(node ChordNode, key string, mode faultMode) (string, error) {
	switch mode {
	case noFault:
		ip, err := node.node.Get(key)
		return string(ip), err
	case intermittentFault:
		//code
		ip, err := node.node.Get(key)
		return string(ip), err
	case permanentFault:
		// some codee here
	}
	ip, err := node.node.Get(key)
	return string(ip), err
}

// resolveDomain handles domain resolution with retries and fault simulation
func resolveDomain(nodes []ChordNode, domain string, wg *sync.WaitGroup, results chan<- string, durations chan<- time.Duration, logFile *os.File, mode faultMode) {
	defer wg.Done()
	var elapsed time.Duration
	var success bool
	var legal bool

	hashedDomain := chord.HashKey(domain).String()

	for i := 0; i < maxRetries; i++ {
		node := nodes[rand.Intn(len(nodes))] // Choose a random node
		start := time.Now()

		legal = isLegalDomain(domain)
		if !legal {
			break
		}

		ip, err := faultInjectedGet(node, hashedDomain, mode)
		elapsed = time.Since(start)

		if err == nil {
			logEntry := fmt.Sprintf("Node: %v, Domain: %s, Resolved IP: %s, Elapsed Time: %v\n", node, domain, ip, elapsed)
			results <- logEntry
			logFile.WriteString(logEntry)
			if durations != nil {
				durations <- elapsed
			}
			success = true
			break
		} else {
			logEntry := fmt.Sprintf("Node: %v, Domain: %s, Attempt: %d, Error: %v\n", node, domain, i+1, err)
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

// runAutomatedTestOnce runs one iteration of the automated test with the current concurrency setting
func runAutomatedTestOnce(nodes []ChordNode, logFile *os.File, mode faultMode) {
	var wg sync.WaitGroup
	results := make(chan string, len(automatedDomainList)*maxRetries)
	durations := make(chan time.Duration, len(automatedDomainList)*maxRetries)

	for i, domain := range automatedDomainList {
		if i%concurrentRequests == 0 {
			wg.Wait()
		}
		wg.Add(1)
		go resolveDomain(nodes, domain, &wg, results, durations, logFile, mode)
	}

	wg.Wait()
	close(results)
	close(durations)

	for result := range results {
		fmt.Println(result)
	}
}

// automatedTesting performs automated testing on a list of domains (no fault)
func automatedTesting(nodes []ChordNode) {
	logFile, err := os.OpenFile("./results/automated.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
	}
	defer logFile.Close()

	for {
		runAutomatedTestOnce(nodes, logFile, noFault)
		time.Sleep(interval)
	}
}

// intermittentFaultTesting simulates intermittent faults (node sleep/wake cycles)
func intermittentFaultTesting(nodes []ChordNode) {
	logFile, err := os.OpenFile("./results/intermittent_fault.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}
	defer logFile.Close()

	fmt.Println("Running Intermittent Fault Test with concurrency = 3")
	concurrentRequests = 3
	runAutomatedTestOnce(nodes, logFile, intermittentFault)
	concurrentRequests = defaultConcurrentRequests
}

// permanentFaultTesting simulates a permanent fault (node sleeps forever)
func permanentFaultTesting(nodes []ChordNode) {
	logFile, err := os.OpenFile("./results/permanent_fault.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}
	defer logFile.Close()

	fmt.Println("Running Permanent Fault Test with concurrency = 3")
	concurrentRequests = 3
	runAutomatedTestOnce(nodes, logFile, permanentFault)
	concurrentRequests = defaultConcurrentRequests
}

// manualResolution allows manual testing
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
		durations := make(chan time.Duration, 1)

		wg.Add(1)
		go resolveDomain(nodes, domain, &wg, results, durations, logFile, noFault)

		wg.Wait()
		close(results)
		close(durations)
		for result := range results {
			fmt.Println(result)
		}
	}
}

// The experiment code from the previous step (4 in the menu) is omitted for brevity
// but would remain the same as previously described.

func runClientInterface(nodes []ChordNode) {
	rand.Seed(time.Now().UnixNano())
	for {
		fmt.Println("\nMain Menu:")
		fmt.Println("1. Manual Domain Name Resolution")
		fmt.Println("2. Automated Testing (No Fault)")
		fmt.Println("3. Exit Program")
		fmt.Println("4. Automated Testing Experiment (Vary Concurrency)")
		fmt.Println("5. Intermittent Fault Test (concurrentRequests=3)") //manually varied the concurrent requests to 5, 7
		fmt.Println("6. Permanent Fault Test (concurrentRequests=3)")

		var choice int
		fmt.Print("Enter your choice: ")
		fmt.Scanln(&choice)

		switch choice {
		case 1:
			manualResolution(nodes)
			return
		case 2:
			automatedTesting(nodes)
			return
		case 3:
			fmt.Println("Exiting program.")
			os.Exit(0)
		case 4:
			// Call the existing experiment function here
			// automatedTestingExperiment(nodes)
			fmt.Println("Run your concurrency experiment here...")
		case 5:
			intermittentFaultTesting(nodes)
		case 6:
			permanentFaultTesting(nodes)
		default:
			fmt.Println("Invalid choice. Please try again.")
		}
	}
}
