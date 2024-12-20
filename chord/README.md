# Chord Implementation

This directory contains the implementation of the Chord protocol, a distributed hash table (DHT) designed for efficient and scalable lookups in a peer-to-peer network. The Chord ring organizes nodes and data into a logical ring structure, enabling logarithmic-time routing and fault tolerance.

## Features
### Key Components
1. Configurable Parameters (`config.go`)
   - **Identifier Space:** 10-bit identifier space (1024 keys/nodes).
   - **Number of Successors:** Configurable for fault tolerance (default: 4).
   - **Replication Factor:** Determines the number of replicas for each key (default: 5).

2. Hashing (`utils.go`)
   - Consistent hashing function to map node addresses and keys to the Chord ring.
   - Modular arithmetic ensures balanced key distribution.

3. Nodes (`node.go`)
   - Nodes manages routing, data storage, and communication within the Chord ring.
   - Implements stabilization, replication, and fault recovery.

4. Finger Table (`finger_table.go`)
   - Efficient routing through logarithmic-time lookups using precomputed intervals.
   - Periodic maintenance to adapt to changes in the ring.

5. Successors and Predecessor Management (`node.go`)
   - Maintains a list of successors for fault tolerance.
   - Updates predecessor and successor pointers during stabilization.

6. Data Storage and Retrieval
   - Supports key-value storage and replication using the `ReplicatedPut` function.
   - Lookups ensure correctness by resolving keys to their responsible nodes.

7. HTTP-Based Inter-Node Communication (`http_client.go`, `http_server.go`)
   - Nodes act as both HTTP servers and clients, enabling flexible communication.
   - Endpoints for key operations: `store`, `retrieve`, `transfer`, `notify`, and `replicate`.

### Fault Tolerance
- **Replication:** Data is replicated across multiple successors to ensure availability.
- **Stabilization:** Periodic processes ensure consistent ring topology and replication integrity.
- **Fault Detection:** Nodes regularly ping successors and predecessors to detect failures.

## File Structure
### Core Files
- `config.go`: Defines global constants and error types for the Chord implementation.
- `node.go`: Implements the core logic for nodes in the Chord ring.
- `finger_table.go`: Manages routing efficiency through finger tables.
- `utils.go`: Provides hashing and utility functions.

### HTTP Communication
- `http_client.go`: Defines client-side HTTP interactions for inter-node communication.
- `http_server.go`: Hosts an HTTP server for nodes to expose operations like lookups and data storage.

### Message Handling
- `messages.go`: Structures for inter-node message exchange.

### Remote Nodes
- `remote_node.go`: Represents and interacts with nodes in the Chord ring using JSON serialization.

## Usage
### Node Initialization
- Each node is initialized with a unique identifier and starts as part of a standalone ring or joins an existing ring using an introducer.

### Finger Table Maintenance
- Finger tables are initialized during node join and periodically updated to ensure routing correctness.

### Replication
- Keys are replicated to a set number of successors based on the replication factor to enhance data availability.

### HTTP Endpoints
- `/ping`: Verifies if a node is alive.
- `/store/{key}`: Stores a key-value pair.
- `/key/{key}`: Retrieves a value for a given key.
- `/store-replica/{key}`: Stores replica data.
- `/notify`: Updates successor or predecessor information.
- `/transfer-keys`: Facilitates key transfers during node join/leave.

## Configuration

| Parameter               | Description                           | Default Value |
|-------------------------|---------------------------------------|---------------|
| `M`                    | Number of bits in the identifier space | `10`          |
| `NumSuccessors`         | Number of successors for fault tolerance | `4`           |
| `ReplicationFactor`     | Number of replicas for each key       | `5`           |
| `StabilizeInterval`     | Time interval for stabilization       | `500ms`       |
| `FixFingersInterval`    | Time interval for finger table maintenance | `500ms`       |
| `CheckPredInterval`     | Time interval for predecessor checks  | `3s`          |

## Error Handling

- **Key Not Found:** Raised if a key does not exist in the DHT.
- **Node Not Found:** Indicates a missing node during lookups.
- **Operation Timeout:** Signals delays in network communication.

## Future Enhancements
- Dynamic stabilization intervals.
- A bootstrap list of introducer nodes, chosen at random with load balancing to initiate node join.
