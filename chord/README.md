# Files
`config.go`  
Contains constants and variables.  

`node.go`  
Contains `Node` struct definition and core functions.  
- `NewNode` - to initialise a node
- `Join` - to allow a new node to join the ring
- `FindSucessor` - 
- `Get`
- `Put`
- `stabilize()`
- `startStabilize()`

`remote_node.go`  
Contains the concept of remote node and interactions over the 
network.

`http_server.go`  
Hosts an HTTP server for the node.
- Exposes endpoints like `/ping`, `/successor`, `/predecessor` with handler functions to allow other nodes to ping this node, find successor, and get predecessor.

`http_client.go`  
Allows a node to send HTTP req to other nodes' HTTP servers.
- `Ping` - to check if a remote node is alive.
- `FindSuccessor`
- `GetPredecessor`
- `Notify`

`http_server.go`


`messages.go`  
Contains `NodeMsg` definition and other message related stuff.

`finger_table.go`  
Contains finger table and routing.

`utils.go`  
Contains utility functions.
- `hashKey` - to generate consistent hashing i.e. same hashes for same input, and different hashes for different inputs.
- `between` - to check if an ID is in between two IDs.

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

## Next steps
According to Claude, these are the next steps in order of priority:  
1) Periodic Maintenance Routines  
    - stabilise() to periodically verify and update successor pointers  
    - fixFingers() to periodically refresh finger table entries
    - checkPredecessor() to detect node failures

2) HTTP Transport Layer
    - Create an HTTP implementation of the NodeClient interface
    - Set up HTTP endpoints for all required RPCs (FindSuccessor, GetKey, StoreKey, etc.)
    - Implement request/response handling for node communication

3) Key Replication and Consistency
    - Complete the replicateKey() implementation
    - Add version tracking for keys (no need for our proj I think)
    - Implement periodic key synchronization between replicas (no need for our proj I think)

4) Failure Detection and Recovery (which I think should be last step instead)
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
2) To see test outputs, from the parent directory run
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