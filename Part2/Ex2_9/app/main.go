package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
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

const homePage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Todo app</title>
<style>
body { font: 1rem system-ui, sans-serif; margin: 0 auto; max-width: 48rem; padding: 1rem; }
img { height: auto; max-width: 100%; }
form { display: flex; gap: .5rem; margin: 1rem 0; }
input { flex: 1; min-width: 0; padding: .6rem; }
button { padding: .6rem 1rem; }
#todo-error { color: #b00020; }
</style>
</head>
<body>
<main>
<h1>Todo app</h1>
<img src="/image" alt="Random image from Lorem Picsum">
<form id="todo-form">
<label for="todo">New todo</label>
<input id="todo" name="todo" type="text" maxlength="2048" required>
<button type="submit">Send</button>
</form>
<p id="todo-error" role="alert"></p>
<h2>Todos</h2>
<ul>
{{range .}}<li>{{.}}</li>
{{else}}<li>No todos yet</li>{{end}}
</ul>
</main>
<script>
document.getElementById("todo-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const input = document.getElementById("todo");
  const error = document.getElementById("todo-error");
  error.textContent = "";
  try {
    const response = await fetch("/todos", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ todo: input.value })
    });
    if (!response.ok) {
      error.textContent = await response.text();
      return;
    }
    window.location.reload();
  } catch (requestError) {
    error.textContent = "Could not save todo. Please try again.";
  }
});
</script>
</body>
</html>`

var pageTemplate = template.Must(template.New("home").Parse(homePage))

type cacheState struct {
	FetchedAt   time.Time `json:"fetched_at"`
	StaleServed bool      `json:"stale_served"`
}

type imageCache struct {
	dir      string
	imageURL string
	ttl      time.Duration
	client   *http.Client
	now      func() time.Time
	mu       sync.Mutex
}

func newImageCache(dir, imageURL string, ttl, requestTimeout time.Duration) *imageCache {
	return &imageCache{
		dir:      dir,
		imageURL: imageURL,
		ttl:      ttl,
		client:   &http.Client{Timeout: requestTimeout},
		now:      time.Now,
	}
}

func fetchTodos(ctx context.Context, client *http.Client, backendURL string) ([]string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, backendURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create todo backend request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch todos: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch todos: unexpected status %s", response.Status)
	}

	var todos []string
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&todos); err != nil {
		return nil, fmt.Errorf("decode todos: %w", err)
	}
	return todos, nil
}

func todoProxyHandler(client *http.Client, backendURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			todos, err := fetchTodos(r.Context(), client, backendURL)
			if err != nil {
				log.Printf("fetch todos: %v", err)
				http.Error(w, "todo service is unavailable", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(todos); err != nil {
				log.Printf("write todos response: %v", err)
			}
		case http.MethodPost:
			request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, backendURL, r.Body)
			if err != nil {
				log.Printf("create todo request: %v", err)
				http.Error(w, "todo service is unavailable", http.StatusServiceUnavailable)
				return
			}
			request.Header.Set("Content-Type", r.Header.Get("Content-Type"))
			response, err := client.Do(request)
			if err != nil {
				log.Printf("create todo: %v", err)
				http.Error(w, "todo service is unavailable", http.StatusServiceUnavailable)
				return
			}
			defer response.Body.Close()
			if contentType := response.Header.Get("Content-Type"); contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
			w.WriteHeader(response.StatusCode)
			if _, err := io.Copy(w, io.LimitReader(response.Body, 4<<10)); err != nil {
				log.Printf("write create todo response: %v", err)
			}
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
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
	if c.now().Sub(state.FetchedAt) < c.ttl {
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
	port := requiredEnv("PORT")
	dataDir := requiredEnv("DATA_DIR")
	imageURL := requiredURL("IMAGE_URL")
	todoBackendURL := requiredURL("TODO_BACKEND_URL")
	imageCacheTTL := requiredDuration("IMAGE_CACHE_TTL")
	imageRequestTimeout := requiredDuration("IMAGE_REQUEST_TIMEOUT")
	todoRequestTimeout := requiredDuration("TODO_REQUEST_TIMEOUT")
	shutdownTimeout := requiredDuration("SHUTDOWN_TIMEOUT")
	if parsedPort, err := strconv.Atoi(port); err != nil || parsedPort < 1 || parsedPort > 65535 {
		log.Fatalf("invalid PORT %q: must be an integer from 1 to 65535", port)
	}
	todoClient := &http.Client{Timeout: todoRequestTimeout}

	cache := newImageCache(dataDir, imageURL, imageCacheTTL, imageRequestTimeout)
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
	mux.HandleFunc("/todos", todoProxyHandler(todoClient, todoBackendURL))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		todos, err := fetchTodos(r.Context(), todoClient, todoBackendURL)
		if err != nil {
			log.Printf("fetch todos: %v", err)
			http.Error(w, "todo service is unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := pageTemplate.Execute(w, todos); err != nil {
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
				ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
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

func requiredEnv(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		log.Fatalf("%s must be set", name)
	}
	return value
}

func requiredURL(name string) string {
	value := requiredEnv(name)
	parsed, err := http.NewRequest(http.MethodGet, value, nil)
	if err != nil || parsed.URL.Host == "" || (parsed.URL.Scheme != "http" && parsed.URL.Scheme != "https") {
		log.Fatalf("%s must be an absolute HTTP or HTTPS URL", name)
	}
	return value
}

func requiredDuration(name string) time.Duration {
	value := requiredEnv(name)
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		log.Fatalf("%s must be a positive duration", name)
	}
	return duration
}
