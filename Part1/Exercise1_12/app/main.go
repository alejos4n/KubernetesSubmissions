package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const imageTTL = 10 * time.Minute

type cacheState struct {
	FetchedAt   time.Time `json:"fetched_at"`
	StaleServed bool      `json:"stale_served"`
}

type imageCache struct {
	dir      string
	imageURL string
	client   *http.Client
	now      func() time.Time
	mu       sync.Mutex
}

func newImageCache(dir, imageURL string) *imageCache {
	return &imageCache{
		dir:      dir,
		imageURL: imageURL,
		client:   &http.Client{Timeout: 30 * time.Second},
		now:      time.Now,
	}
}

func (c *imageCache) imagePath() string {
	return filepath.Join(c.dir, "image.jpg")
}

func (c *imageCache) statePath() string {
	return filepath.Join(c.dir, "state.json")
}

func (c *imageCache) get() ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	image, imageErr := os.ReadFile(c.imagePath())
	stateData, stateErr := os.ReadFile(c.statePath())
	if errors.Is(imageErr, os.ErrNotExist) && errors.Is(stateErr, os.ErrNotExist) {
		freshImage, err := c.fetch()
		if err != nil {
			return nil, false, err
		}
		if err := c.store(freshImage, cacheState{FetchedAt: c.now().UTC()}); err != nil {
			return nil, false, err
		}
		return freshImage, false, nil
	}
	if imageErr != nil {
		return nil, false, fmt.Errorf("read cached image: %w", imageErr)
	}
	if stateErr != nil {
		return nil, false, fmt.Errorf("read image cache state: %w", stateErr)
	}

	var state cacheState
	if err := json.Unmarshal(stateData, &state); err != nil {
		return nil, false, fmt.Errorf("decode image cache state: %w", err)
	}
	if state.FetchedAt.IsZero() {
		return nil, false, errors.New("image cache state has no fetch timestamp")
	}
	if c.now().Sub(state.FetchedAt) < imageTTL {
		return image, false, nil
	}
	if !state.StaleServed {
		state.StaleServed = true
		if err := c.writeState(state); err != nil {
			return nil, false, err
		}
		return image, true, nil
	}

	freshImage, err := c.fetch()
	if err != nil {
		return image, true, err
	}
	if err := c.store(freshImage, cacheState{FetchedAt: c.now().UTC()}); err != nil {
		return nil, false, err
	}
	return freshImage, false, nil
}

func (c *imageCache) fetch() ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, c.imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create image request: %w", err)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download image: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download image: unexpected status %s", response.Status)
	}

	const maxImageSize = 20 << 20
	image, err := io.ReadAll(io.LimitReader(response.Body, maxImageSize+1))
	if err != nil {
		return nil, fmt.Errorf("read downloaded image: %w", err)
	}
	if len(image) == 0 || len(image) > maxImageSize {
		return nil, fmt.Errorf("downloaded image size is invalid: %d bytes", len(image))
	}
	return image, nil
}

func (c *imageCache) store(image []byte, state cacheState) error {
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return fmt.Errorf("create image cache directory: %w", err)
	}
	imageTemp := c.imagePath() + ".tmp"
	if err := os.WriteFile(imageTemp, image, 0644); err != nil {
		return fmt.Errorf("write cached image: %w", err)
	}
	if err := os.Rename(imageTemp, c.imagePath()); err != nil {
		return fmt.Errorf("publish cached image: %w", err)
	}
	return c.writeState(state)
}

func (c *imageCache) writeState(state cacheState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode image cache state: %w", err)
	}
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return fmt.Errorf("create image cache directory: %w", err)
	}
	stateTemp := c.statePath() + ".tmp"
	if err := os.WriteFile(stateTemp, data, 0644); err != nil {
		return fmt.Errorf("write image cache state: %w", err)
	}
	if err := os.Rename(stateTemp, c.statePath()); err != nil {
		return fmt.Errorf("publish image cache state: %w", err)
	}
	return nil
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if _, err := strconv.Atoi(port); err != nil {
		log.Fatalf("invalid PORT %q: %v", port, err)
	}
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}
	imageURL := os.Getenv("IMAGE_URL")
	if imageURL == "" {
		imageURL = "https://picsum.photos/1200"
	}

	cache := newImageCache(dataDir, imageURL)
	mux := http.NewServeMux()
	mux.HandleFunc("/image", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		image, stale, err := cache.get()
		if err != nil && image == nil {
			log.Printf("get cached image: %v", err)
			http.Error(w, "image is unavailable", http.StatusServiceUnavailable)
			return
		}
		if err != nil {
			log.Printf("refresh cached image: %v", err)
		}
		if stale {
			w.Header().Set("Warning", `110 - "Response is stale"`)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(image); err != nil {
			log.Printf("write image response: %v", err)
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, err := io.WriteString(w, `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Project image</title></head>
<body><main><h1>Project image</h1><img src="/image" alt="Random image from Lorem Picsum" style="max-width:100%;height:auto"></main></body>
</html>`)
		if err != nil {
			log.Printf("write page response: %v", err)
		}
	})

	server := &http.Server{Addr: net.JoinHostPort("", port), Handler: mux}
	if strings.EqualFold(os.Getenv("ENABLE_SHUTDOWN"), "true") {
		mux.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			fmt.Fprintln(w, "shutting down")
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := server.Shutdown(ctx); err != nil {
					log.Printf("shut down HTTP server: %v", err)
				}
			}()
		})
	}

	log.Printf("Server started on port %s", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve HTTP requests: %v", err)
	}
}
