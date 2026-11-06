package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

type todoRequest struct {
	Todo string `json:"todo"`
}

type todoStore struct {
	mu    sync.RWMutex
	todos []string
}

func newTodoHandler() http.Handler {
	store := &todoStore{
		todos: []string{
			"Read the Kubernetes documentation",
			"Build a useful todo app",
			"Take a short break",
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/todos", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/todos" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			store.mu.RLock()
			todos := append([]string(nil), store.todos...)
			store.mu.RUnlock()
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(todos); err != nil {
				log.Printf("encode todos: %v", err)
			}
		case http.MethodPost:
			r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
			var request todoRequest
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&request); err != nil {
				var maxBytesError *http.MaxBytesError
				if errors.As(err, &maxBytesError) {
					http.Error(w, "request body is too large", http.StatusRequestEntityTooLarge)
					return
				}
				http.Error(w, "request body must be valid JSON with a todo field", http.StatusBadRequest)
				return
			}
			var extra any
			if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
				http.Error(w, "request body must contain one JSON object", http.StatusBadRequest)
				return
			}
			todo := strings.TrimSpace(request.Todo)
			if todo == "" {
				http.Error(w, "todo must not be empty", http.StatusBadRequest)
				return
			}
			if utf8.RuneCountInString(todo) > 140 {
				http.Error(w, "todo must be 140 characters or fewer", http.StatusBadRequest)
				return
			}

			store.mu.Lock()
			store.todos = append(store.todos, todo)
			store.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			if err := json.NewEncoder(w).Encode(todoRequest{Todo: todo}); err != nil {
				log.Printf("encode created todo: %v", err)
			}
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func main() {
	port, ok := os.LookupEnv("PORT")
	if !ok || port == "" {
		log.Fatal("PORT must be set")
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		log.Fatalf("invalid PORT %q: must be an integer from 1 to 65535", port)
	}
	server := &http.Server{Addr: net.JoinHostPort("", port), Handler: newTodoHandler()}
	log.Printf("Todo backend listening on port %s", port)
	log.Fatal(server.ListenAndServe())
}
