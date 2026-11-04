package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type logOutput struct {
	value       string
	pingPongURL string
	client      *http.Client
	mu          sync.RWMutex
	line        string
}

func newLogOutput(pingPongURL string) (*logOutput, error) {
	value, err := randomUUID()
	if err != nil {
		return nil, err
	}
	output := &logOutput{
		value:       value,
		pingPongURL: pingPongURL,
		client:      &http.Client{Timeout: 2 * time.Second},
	}
	output.updateLine()
	return output, nil
}

func (o *logOutput) updateLine() {
	line := fmt.Sprintf("%s: %s.", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), o.value)
	o.mu.Lock()
	o.line = line
	o.mu.Unlock()
}

func (o *logOutput) latestLine() string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.line
}

func (o *logOutput) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, o.pingPongURL, nil)
	if err != nil {
		http.Error(w, "invalid Ping-pong URL", http.StatusInternalServerError)
		return
	}
	response, err := o.client.Do(request)
	if err != nil {
		log.Printf("request Ping-pong count: %v", err)
		http.Error(w, "Ping-pong service is unavailable", http.StatusServiceUnavailable)
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		log.Printf("Ping-pong count request returned %s", response.Status)
		http.Error(w, "Ping-pong service returned an error", http.StatusBadGateway)
		return
	}

	count, err := io.ReadAll(io.LimitReader(response.Body, 32))
	if err != nil {
		log.Printf("read Ping-pong count: %v", err)
		http.Error(w, "could not read Ping-pong count", http.StatusBadGateway)
		return
	}
	countValue := strings.TrimSpace(string(count))
	if countValue == "" {
		http.Error(w, "Ping-pong returned an empty count", http.StatusBadGateway)
		return
	}
	if _, err := strconv.ParseUint(countValue, 10, 64); err != nil {
		log.Printf("invalid Ping-pong count %q: %v", countValue, err)
		http.Error(w, "Ping-pong returned an invalid count", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "%s\nPing / Pongs: %s\n", o.latestLine(), countValue)
}

func main() {
	pingPongURL := os.Getenv("PINGPONG_URL")
	if pingPongURL == "" {
		pingPongURL = "http://ping-pong/pings"
	}
	output, err := newLogOutput(pingPongURL)
	if err != nil {
		log.Fatalf("create log output: %v", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			output.updateLine()
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", output.serveHTTP)
	server := &http.Server{Addr: ":8080", Handler: mux}
	log.Printf("Log output listening on :8080")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve Log output: %v", err)
	}
}

func randomUUID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("generate random UUID: %w", err)
	}

	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80

	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], id[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], id[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], id[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], id[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], id[10:16])

	return string(encoded), nil
}
