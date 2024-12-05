# Distributed-Systems-Chord-DNS



## Set Up
In the parent directory, run
```bash
docker-compose -f docker/docker-compose.yml up --build
```



## Files
* __Dockerfile__: Creates a container environment for the DNS service.
* __docker-compose.yml__: Configures _ instances of the service in a distributed network system, where each instance is accessible via a unique port on localhost.
    * Node 1 is accessible at http://localhost:8081
    * ...
    * Node 3 is accessible at http://localhost:8083



## Shut Down

```bash
docker-compose down # Stop the containers
```


# Updated
1) docker-compose up --build

In another terminal,  
2) docker exec -it chord-dns-node0 sh

3)
curl http://node0:8001/ping
curl http://node1:8001/ping
curl http://node2:8001/ping

4) 
curl http://node0:8001/successors
curl http://node1:8001/successors
curl http://node2:8001/successors

5) "who is responsible for ID 100?"
curl http://node0:8001/successor/100