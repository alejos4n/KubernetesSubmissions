package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

func main() {
	var requests atomic.Uint64

	mux := http.NewServeMux()
	mux.HandleFunc("/pingpong", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		fmt.Fprintf(w, "pong %d\n", requests.Add(1)-1)
	})

	log.Fatal(http.ListenAndServe(":8080", mux))
}
