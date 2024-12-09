# Distributed Systems Chord DNS

## Overview
This project implements a distributed DNS service using the Chord protocol. The service allows nodes to join a network, store DNS records, and resolve domain names through a distributed hash table (DHT). The implementation includes a test suite to validate the functionality of the Chord nodes and their interactions.

## Set Up
To run the application, ensure you have Go installed. In the parent directory, run:

## Files
- **`config.go`**: Contains constants and variables.
- **`node.go`**: Contains the `Node` struct definition and core functions.
  - `NewNode`: Initializes a node.
  - `Join`: Allows a new node to join the ring.
  - `Find`: Locates the node responsible for a specific key.
  - `Get`: Retrieves the value associated with a key by finding the responsible node.
  - `Put`: Stores a key-value pair on the node responsible for the key's hash, either locally or by forwarding the request to the correct node.
  - `stabilize()`: Periodically checks and updates the node's successor and notifies it of the node’s presence.
  - `startStabilize()`: Starts a periodic loop that calls stabilize at regular intervals.
  - `fixFingers()`: Periodically refreshes entries in the finger table to ensure they point to the correct successors.
  - `startFixFingers()`: Starts a periodic loop that calls fixFingers at regular intervals to maintain the finger table.
- **`remote_node.go`**: Contains the concept of remote nodes and interactions over the network.
- **`http_server.go`**: Hosts an HTTP server for the node.
  - Exposes endpoints like `/ping`, `/successor`, `/predecessor` with handler functions to allow other nodes to ping this node, find successors, and get predecessors.
- **`http_client.go`**: Allows a node to send HTTP requests to other nodes' HTTP servers.
  - `Ping`: Checks if a remote node is alive.
  - `FindSuccessor`: Locates the successor node responsible for a given ID by sending an HTTP request to the target node.
  - `GetPredecessor`: Retrieves the predecessor of the target node by sending an HTTP request to its server.
  - `Notify`: Notifies a target node of this node's presence by sending a POST request with this node's info, necessary for stabilization.
  - `StoreKey`: Forwards a key-value pair to the target node for storage via an HTTP POST request.
  - `GetKey`: Retrieves a key’s value and version from the target node by sending a GET request.
  - `DeleteKey`: To be implemented in future work.
  - `TransferKeys`: To be implemented in future work.
- **`messages.go`**: Contains `NodeMsg` definition and other message-related structures.
- **`finger_table.go`**: Contains finger table and routing logic.
- **`utils.go`**: Contains utility functions.
  - `HashKey`: Generates consistent hashing (i.e., same hashes for the same input, and different hashes for different inputs).
  - `between`: Checks if an ID is in between two IDs.
  - `CompareNodes`: Compares two node IDs and returns whether the first is "less than," "greater than," or "equal to" the second.
- **`main.go`**: Contains the main application logic and local tests.
  - `testPing`: Tests if each node's HTTP server is running and responds to ping requests.
  - `testNodeJoining`: Tests if a new node can join the ring through an introducer node.
  - `testStabilization`: Tests if nodes update successors and predecessors correctly over multiple iterations.
  - `testPutAndGet`: Tests if key-value pairs can be stored and retrieved correctly.
  - `testFingerTable`: Tests if the finger table entries for each node are correctly populated.


## Working with the Code
We can focus on local implementation and ensure the code works before incorporating Docker. 

### Testing the Implementation
To run the test suite, you can use the following commands:

- To run all tests:
  ```bash
  go run main.go -all
  ```

- To run specific tests:
  ```bash
  go run main.go -join -fingers -stabilize -ops -dht
  ```
### Testing Interactive DNS Operations

The interactive DNS test (interactiveDNS) simulates the experience of entering DNS records (domain-to-IP mappings) and retrieving them interactively. This test demonstrates how users can dynamically store and query DNS records within the distributed Chord network.

To use the interactive DNS test, follow these steps:

Run the Interactive Test: Start the application in interactive mode. The test will allow you to enter commands to add and retrieve DNS records manually.
- To run interactive DNS lookup:
  ```bash
  go run main.go -interactive
  ```

Commands: In interactive mode, you can use the following commands:

Put Operation: Stores a DNS record (domain-to-IP mapping) in the Chord network.

- To put domain name and ip pair:
  ```bash
  put <domain> <ip_address>
  ```
    Example: put google.com 172.217.20.206
    Output: Confirms the record has been stored, showing the hash, IP, and node responsible for storage.

Get Operation: Retrieves the IP address for a domain.
- To get domain name and ip pair:

  ```bash
  get <domain>
  ```
    Example: get google.com
    Output: Displays the retrieved IP address or an error if the domain does not exist in the DHT.

### Available Tests
- **Join Test**: Tests if a node can successfully join the Chord ring.
- **Finger Table Test**: Verifies that the finger tables of the nodes are correctly populated.
- **Stabilization Test**: Ensures that nodes update their successors and predecessors correctly over time.
- **Put and Get Operations Test**: Simulates storing and retrieving DNS records, demonstrating how users would input domain names and their corresponding IP addresses.
- **DHT Test**: Prints the distributed hash tables for each node.
- **interactiveDNS**: Simulate Client Interaction for DNS lookups

## Node Joining Process
When a node starts, it initializes itself and attempts to join the Chord ring. The first node creates a new ring, while subsequent nodes join through an introducer node. Each node is assigned a unique address based on the port it runs on, allowing it to act as both a server and a client for handling HTTP requests.

## DNS Resolution
The `testPutAndGet` function simulates how users would input domain names and their corresponding IP addresses. The DNS queries are resolved using the Chord lookup mechanism, which finds the responsible node for a given domain.
The `interactiveDNS` function interacts with the client who will store these key-value pairs in the Chord ring, allowing for efficient retrieval. The DNS queries are resolved using the Chord lookup mechanism, which finds the responsible node for a given domain.

## Future Work
Currently, the DNS application (`dns_app`) is not interacting with the Chord ring properly. Future enhancements will focus on:
- Implementing caching mechanisms to improve performance.
- Adding fault tolerance features, such as key replication, to ensure data integrity and availability in case of node failures.
- Enhancing the interaction between the DNS resolver and the Chord network to allow seamless DNS queries.

## Conclusion
This project serves as a foundation for building a distributed DNS service using the Chord protocol. The current implementation provides basic functionality, and future work will enhance its robustness and efficiency.
2) HTTP Transport Layer
    - Set up HTTP endpoints and implement request/response handling for DeleteKey and TransferKey (by Checkpoint 3)

3) Key Replication and Consistency (by Checkpoint 3)
    - Complete the replicateKey() implementation
    - Add version tracking for keys
    - Implement periodic key synchronization between replicas

4) Failure Detection and Recovery (by Checkpoint 3)
    - Add heartbeat mechanisms between nodes
    - Implement successor list maintenance
    - Add key recovery when nodes fail

5) DNS Integration
    - Create DNS record structure
    - Implement DNS lookup using the Chord DHT
    - Add TTL handling for DNS records

