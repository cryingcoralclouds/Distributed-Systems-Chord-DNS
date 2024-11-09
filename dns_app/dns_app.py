# running the file python dns_app.py --chord-nodes="http://localhost:8001,http://localhost:8002" 
# OR --seed-file="seed_nodes.json" 
from flask import Flask
import requests
import hashlib
import click
import json
from cachetools import TTLCache, LRUCache
from typing import Dict, Optional, List
from datetime import datetime
from threading import Lock

app = Flask(__name__)

class ModuloLoadBalancer: ## for the cache 
    def __init__(self, nodes: List[str]):
        self.nodes = nodes
        self.current_index = 0
        self.lock = Lock()  # Thread-safe counter increment
        
    def get_next_node(self) -> str:
        """Get the next node using modulo wrapping."""
        with self.lock:
            node = self.nodes[self.current_index]
            self.current_index = (self.current_index + 1) % len(self.nodes)
            return node
            
    def get_node_status(self) -> Dict:
        """Return current load balancer status."""
        return {
            "total_nodes": len(self.nodes),
            "current_index": self.current_index,
            "available_nodes": self.nodes
        }

class DNSCache:
    def __init__(self, ttl_seconds: int = 300, max_size: int = 5):
        self.ttl_cache = TTLCache(maxsize=max_size, ttl=ttl_seconds)
        self.lru_cache = LRUCache(maxsize=max_size)
        
    def get(self, domain: str) -> Optional[str]:
        result = self.ttl_cache.get(domain)
        if result:
            self.lru_cache[domain] = result
            return result
            
        return self.lru_cache.get(domain)
    
    def put(self, domain: str, ip_address: str):
        self.ttl_cache[domain] = ip_address
        self.lru_cache[domain] = ip_address

    def bulk_insert(self, entries: Dict[str, str]):
        for domain, ip in entries.items():
            self.put(domain, ip)
            
    def get_cache_stats(self) -> Dict:
        return {
            "ttl_cache_size": len(self.ttl_cache),
            "lru_cache_size": len(self.lru_cache),
            "ttl_cache_maxsize": self.ttl_cache.maxsize,
            "lru_cache_maxsize": self.lru_cache.maxsize
        }

class ChordDNSResolver:
    def __init__(self, chord_nodes: list, cache_ttl: int = 300, seed_nodes: Dict[str, str] = None):
        self.load_balancer = ModuloLoadBalancer(chord_nodes)
        self.cache = DNSCache(ttl_seconds=cache_ttl)
        
        # Initialize cache with seed nodes if provided
        if seed_nodes:
            self.initialize_seed_nodes(seed_nodes)
            
    def initialize_seed_nodes(self, seed_nodes: Dict[str, str]):
        self.cache.bulk_insert(seed_nodes)
        click.echo(f"Initialized cache with {len(seed_nodes)} seed entries")
        click.echo("Cache stats after initialization:")
        click.echo(json.dumps(self.cache.get_cache_stats(), indent=2))
        
    def hash_domain(self, domain: str) -> str: #REPLACE WITH OUR HASH
        return hashlib.sha256(domain.encode()).hexdigest()
    
    def resolve_domain(self, domain: str) -> Dict:
        # Check cache first
        cached_result = self.cache.get(domain)
        if cached_result:
            return {
                "domain": domain,
                "ip": cached_result,
                "source": "cache",
                "timestamp": datetime.now().isoformat(),
                "load_balancer_status": self.load_balancer.get_node_status()
            }
        
        try:
            # Hash the domain
            domain_hash = self.hash_domain(domain)
            
            # Get next node using modulo load balancer
            chord_node = self.load_balancer.get_next_node()
            
            # Make API request to Chord network --> WHAT IS THE EXACT API REQUEST
            response = requests.get(
                f"{chord_node}/lookup",
                params={"key": domain_hash}
            )
            
            if response.status_code == 200:
                result = response.json()
                ip_address = result.get("value")
                
                # Store in cache
                self.cache.put(domain, ip_address)
                
                return {
                    "domain": domain,
                    "ip": ip_address,
                    "source": "chord",
                    "node_used": chord_node,
                    "timestamp": datetime.now().isoformat(),
                    "load_balancer_status": self.load_balancer.get_node_status()
                }
            else:
                return {
                    "domain": domain,
                    "error": "Domain not found",
                    "node_used": chord_node,
                    "timestamp": datetime.now().isoformat(),
                    "load_balancer_status": self.load_balancer.get_node_status()
                }
                
        except requests.RequestException as e:
            return {
                "domain": domain,
                "error": f"Failed to resolve: {str(e)}",
                "timestamp": datetime.now().isoformat(),
                "load_balancer_status": self.load_balancer.get_node_status()
            }

def load_seed_nodes(seed_file: str) -> Dict[str, str]:
    try:
        with open(seed_file, 'r') as f:
            return json.load(f)
    except FileNotFoundError:
        click.echo(f"Warning: Seed file {seed_file} not found. Starting with empty cache.")
        return {}
    except json.JSONDecodeError:
        click.echo(f"Warning: Invalid JSON in seed file {seed_file}. Starting with empty cache.")
        return {}

@click.command()
@click.option('--chord-nodes', required=True, help='Comma-separated list of Chord node URLs') #CAN INIT WITH OUR SEED NODES
@click.option('--seed-file', default='seed_nodes.json', help='JSON file containing seed DNS entries')
@click.option('--cache-ttl', default=300, help='Cache TTL in seconds')
def main(chord_nodes: str, seed_file: str, cache_ttl: int):
    """DNS resolver CLI using Chord protocol."""
    # Load seed nodes
    seed_nodes = load_seed_nodes(seed_file)
    
    # Initialize resolver with Chord nodes and seed entries
    nodes = chord_nodes.split(',')
    resolver = ChordDNSResolver(nodes, cache_ttl=cache_ttl, seed_nodes=seed_nodes)
    
    while True:
        domain = click.prompt('Enter domain name to resolve (or "exit" to quit)')
        
        if domain.lower() == 'exit':
            break
            
        result = resolver.resolve_domain(domain)
        
        # Pretty print result
        click.echo(json.dumps(result, indent=2))
        
        if not click.confirm('Do you want to resolve another domain?'):
            break

if __name__ == '__main__':
    main()