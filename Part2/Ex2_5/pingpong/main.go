package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

func main() {
	log.Println("Ping-pong listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", newHandler()))
}

func newHandler() http.Handler {
	var pongs atomic.Uint64
	mux := http.NewServeMux()
	mux.HandleFunc("/pingpong", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		fmt.Fprintf(w, "pong %d\n", pongs.Add(1))
	})
	mux.HandleFunc("/pings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "%d\n", pongs.Load())
	})
	return mux
}
