package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	value, err := randomUUID()
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		fmt.Fprintf(w, "%s: %s\n", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), value)
	})

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		fmt.Printf("%s: %s\n", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), value)
		for range ticker.C {
			fmt.Printf("%s: %s\n", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), value)
		}
	}()

	server := &http.Server{
		Addr:    ":8080",
		Handler: http.DefaultServeMux,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
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
