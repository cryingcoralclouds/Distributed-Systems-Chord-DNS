import requests
import concurrent.futures
import random
import string

# List of node URLs (update with Docker service hostnames if necessary)
nodes = ["http://chord-node:8080"]

# Test data domains (same as in the test suite)
domains = [
    "google.com", "wikipedia.org", "reddit.com", "facebook.com", "youtube.com",
    "amazon.com", "twitter.com", "linkedin.com", "instagram.com", "netflix.com",
    "yahoo.com", "microsoft.com", "apple.com"
]

def get_ip_address(node, domain):
    """Query the Chord node for the IP address of a domain."""
    url = f"{node}/key/{domain}"
    try:
        response = requests.get(url)
        if response.status_code == 200:
            return response.json()
        else:
            return {"error": f"Failed to retrieve {domain}: {response.text}"}
    except Exception as e:
        return {"error": str(e)}

if __name__ == "__main__":
    with concurrent.futures.ThreadPoolExecutor(max_workers=10) as executor:
        futures = []
        for domain in domains:
            # Randomly pick a node for each request
            node = random.choice(nodes)
            futures.append(executor.submit(get_ip_address, node, domain))

        # Print results as they are completed
        for future in concurrent.futures.as_completed(futures):
            print(future.result())