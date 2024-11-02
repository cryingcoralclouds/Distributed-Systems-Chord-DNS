# Files
- `config.go` contains constants and variables  
- `node.go` contains `Node` struct definition and core functions.
- `remote_node.go` contains `RemoteNode` and `NodeClient` interface, which handles the concept of remote node and interactions over the network.
- `messages.go` contains `NodeMsg` definition and other message related stuff.
- `finger_table.go` contains finger table and routing.
- `utils.go` contain utility functions.


# Working with the code
## Go and Docker
Note that initialising nodes on Docker and the `NewNode()` function in `chord.go` are two separate things.  
- `NewNode()` will only run if it's explicitly called in your code, usually in the `main()` function.  
- Docker simply starts the container, which executes the command specified in your Dockerfile or docker-compose.yml, often something like go run main.go.  
- To run the functions in `chord.go` like hw1, you can call them in another file e.g. `chord_test.go`.  

How the logic in our Go application relates to Docker
- NewNode() is responsible for creating the logical node in our Go application.  
- Docker containers simulate multiple instances of your application, with each container running a unique node initialized by NewNode().

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
To see logs from node 1, which is named docker-node1-1, from the parent directory run   
```
docker logs -f docker-node1-1
```
To see test outputs, from the parent directory run
```
go test ./chord
```