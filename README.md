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
