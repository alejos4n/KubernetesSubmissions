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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := fmt.Fprint(w, `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Todo app</title>
</head>
<body>
  <main>
    <h1>Todo app</h1>
    <p>Welcome to the todo app.</p>
  </main>
</body>
</html>
`); err != nil {
			log.Printf("write HTTP response: %v", err)
		}
	})

	fmt.Printf("Server started in port %s\n", port)
	if err := http.Serve(listener, nil); err != nil {
		log.Fatalf("serve HTTP requests: %v", err)
	}
}
