package main

import (
	"chord_dns/chord"
	"fmt"
	"log"
)

func createNode(addr string) (*chord.Node, *chord.HTTPNodeServer) {
	client := chord.NewHTTPNodeClient(addr)
	node, err := chord.NewNode(addr, client)
	if err != nil {
		log.Fatalf("Failed to create node at %s: %v", addr, err)
	}

	server := chord.NewHTTPNodeServer(node)
	return node, server
}

func startServer(server *chord.HTTPNodeServer, addr string) {
	go func() {
		if err := server.Start(addr); err != nil {
			log.Printf("Server at %s failed: %v", addr, err)
		}
	}()
}

func printSeparator(title string) {
	fmt.Printf("\n=== %s ===\n\n", title)
}
