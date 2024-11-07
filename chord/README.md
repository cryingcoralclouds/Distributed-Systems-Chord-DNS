# Files
`config.go`  
Contains constants and variables.  

`node.go`  
Contains `Node` struct definition and core functions.  
- `NewNode` - to initialize a node.
- `Join` - to allow a new node to join the ring.
- `A` - to locate the node responsible for a specific key.
- `Get` - Retrieves the value associated with a key by finding the responsible node.
- `Put` - Stores a key-value pair on the node responsible for the key's hash, either locally or by forwarding the request to the correct node.
- `stabilize()` - Periodically checks and updates the node's successor and notifies it of the node’s presence.
- `startStabilize()` - Starts a periodic loop that calls stabilize at regular intervals.
- `fixFingers()` - Periodically refreshes entries in the finger table to ensure they point to the correct successors.
- `startFixFingers()` - Starts a periodic loop that calls fixFingers at regular intervals to maintain the finger table.

`remote_node.go`  
Contains the concept of remote node and interactions over the 
network.

`http_server.go`  
Hosts an HTTP server for the node.
- Exposes endpoints like `/ping`, `/successor`, `/predecessor` with handler functions to allow other nodes to ping this node, find successor, and get predecessor.

`http_client.go`  
Allows a node to send HTTP req to other nodes' HTTP servers.
- `Ping` - to check if a remote node is alive.
- `FindSuccessor` - to locate successor node responsible for a given ID by sending an HTTP request to the target node.
- `GetPredecessor` - to retrieve the predecessor of the target node by sending an HTTP request to its server.
- `Notify` - to nodify a target node of this node's presence by sending a POST request with this node's info, necessary for stabilization.
- `StoreKey` - to forward a key-value pair to the target node for storage via an HTTP POST request.
- `GetKey` - to retrieve a key’s value and version from the target node by sending a GET request.
- `DeleteKey` - to be implemented by Checkpoint 3.
- `TransferKeys` - to be implemented by Checkpoint 3.

`messages.go`  
Contains `NodeMsg` definition and other message related stuff.  

`finger_table.go`  
Contains finger table and routing.  
  
`utils.go`  
Contains utility functions.
- `HashKey` - to generate consistent hashing i.e. same hashes for same input, and different hashes for different inputs.
- `between` - to check if an ID is in between two IDs.  
- `CompareNodes` - to compare two node IDs and returns whether the first is "less than," "greater than," or "equal to" the second

`main.go`  
For now, it contains local tests and no Docker tests yet. Tested with only 2 nodes.
- `testPing` tests if:
    - Each node's HTTP server is running.
    - Each node responds to the ping request with an HTTP 200 status to indicate that it's alive.
- `testNodeJoining` tests if:
    - node2 can join the ring through node1, which acts as an introducer.
    - node2 successfully establishes successor and predecessor.
- `testStabilization` tests if:
    - Nodes update successors and predecessors correctly over multiple iterations.
    - Over time, node1 and node2 maintain accurate knowledge of each other's positions.
    - Nodes' finger table and successor lists are updated correctly.
- `testPutAndGet` tests if:
    - We can store a key-value pair.
    - We can retrieve the value through any node in the ring.
    - For non-existent keys, the correct error is returned.
    - The routing of requests works correctly.
- `testFingerTable` tests if:
    - The finger table entries for each node are correctly populated.
    - It prints the contents of the finger table for both nodes, showing which nodes are referenced in the finger table and if any entries are `nil`.
    - This helps verify that the finger table maintenance is functioning as expected and that nodes have accurate routing information.

# Working with the code
If I'm not wrong, we can focus on local implementation and make sure the code works first before incorporating Docker.
- Use Docker Compose to define multiple containers, each representing a separate node.
- When transitioning from local to Docker, change local host to service name for base URL in `NewHTTPNodeClient`, e.g. from http://localhost:8081 to http://node2:8081

## Go and Docker
Note that initialising nodes on Docker and the `NewNode()` function in `chord.go` are two separate things.  
- `NewNode()` will only run if it's explicitly called in your code, usually in the `main()` function.  
- Docker simply starts the container, which executes the command specified in your Dockerfile or docker-compose.yml, often something like go run main.go.  
- To run the functions in `chord.go` like hw1, you can call them in another file e.g. `chord_test.go`.  

How the logic in our Go application relates to Docker
- NewNode() is responsible for creating the logical node in our Go application.  
- Docker containers simulate multiple instances of your application, with each container running a unique node initialized by NewNode().

Using goroutines for HTTP servers  
- Each node needs to handle incoming HTTP requests from other nodes while also performing background tasks (e.g., stabilizing, fixing finger tables, checking the predecessor).
- Running the HTTP server in a separate goroutine allows each node to accept requests concurrently without blocking other tasks.

## Next Steps
According to Claude, these are the next steps in order of priority:  
1) **Periodic Maintenance Routines**  
    - **fixFingers()**: This function is designed to periodically refresh the entries in the finger table to ensure they point to the correct successors. 
    - **checkPredecessor()**: This function is intended to detect node failures and maintain the integrity of the network.

While these functions are already implemented, they require thorough testing to ensure they operate correctly, especially since they are called within goroutines. 

### Current Challenges
Currently, we are experiencing an issue where the print statements for the finger tables are returning null or blank entries. This could be attributed to several factors:
- **Timing Issues**: The finger table may not be updated in time before the print statements are executed, particularly in a scenario with only two nodes. The limited number of nodes may not provide enough data for the finger table to populate correctly.
- **Update Mechanism**: There may be a problem with how the finger table is being updated, which could lead to incomplete or incorrect entries.

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

## To see outputs in the terminal
1) To run main file, from the parent directory run
    ```
    go run main.go
    ```
2) (Ignore test folder for now, don't really need it) To see test outputs, from the parent directory run
    ```
    go test ./chord
    ```
3) You can also see log outputs on Docker Desktop after running
    ```
    docker-compose -f docker/docker-compose.yml up --build
    ```
4) To see logs from a specific node e.g. node 1, which is named docker-node1-1, from the parent directory run   
    ```
    docker logs -f docker-node1-1
    ```
5) This is a bit extra but if you want try ping another node from the terminal, run
    ```
    go run main.go
    ```
    Then, in another terminal, to ping node with 8001 port, run
    ```
    curl http://localhost:8001/ping
    ```