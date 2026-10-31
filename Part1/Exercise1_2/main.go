package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("start server on port %s: %v", port, err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "todo app")
	})

	fmt.Printf("Server started in port %s\n", port)
	if err := http.Serve(listener, nil); err != nil {
		log.Fatalf("serve HTTP requests: %v", err)
	}
}
