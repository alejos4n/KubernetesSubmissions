package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

func main() {
	const countPath = "/data/pingpong-count"
	if err := os.MkdirAll(filepath.Dir(countPath), 0755); err != nil {
		log.Fatalf("create data directory: %v", err)
	}

	var mu sync.Mutex
	requests := uint64(0)
	count, err := os.ReadFile(countPath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("read request count: %v", err)
	}
	if err == nil {
		requests, err = strconv.ParseUint(strings.TrimSpace(string(count)), 10, 64)
		if err != nil {
			log.Fatalf("parse request count: %v", err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/pingpong", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		mu.Lock()
		next := requests + 1
		tempPath := countPath + ".tmp"
		if err := os.WriteFile(tempPath, []byte(strconv.FormatUint(next, 10)), 0644); err != nil {
			mu.Unlock()
			http.Error(w, fmt.Sprintf("save request count: %v", err), http.StatusInternalServerError)
			return
		}
		if err := os.Rename(tempPath, countPath); err != nil {
			mu.Unlock()
			http.Error(w, fmt.Sprintf("save request count: %v", err), http.StatusInternalServerError)
			return
		}
		requests = next
		mu.Unlock()

		fmt.Fprintf(w, "pong %d\n", next-1)
	})

	log.Fatal(http.ListenAndServe(":8080", mux))
}
