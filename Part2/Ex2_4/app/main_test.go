package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHomePageRendersTodosAndSubmitsToBackend(t *testing.T) {
	var page strings.Builder
	if err := pageTemplate.Execute(&page, []string{"Read Kubernetes", "<safe todo>"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`<input id="todo" name="todo" type="text" maxlength="140" required>`,
		`<button type="submit">Send</button>`,
		"<h2>Todos</h2>",
		"<li>Read Kubernetes</li>",
		"<li>&lt;safe todo&gt;</li>",
		`fetch("/todos"`,
		`<img src="/image"`,
	} {
		if !strings.Contains(page.String(), expected) {
			t.Errorf("rendered home page does not contain %q", expected)
		}
	}
}

func TestFetchTodosUsesBackendEndpoint(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/todos" {
			t.Errorf("unexpected backend request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{"One todo"})
	}))
	defer backend.Close()

	todos, err := fetchTodos(context.Background(), backend.Client(), backend.URL+"/todos")
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 || todos[0] != "One todo" {
		t.Fatalf("todos = %#v, want [One todo]", todos)
	}
}

func TestTodoProxyForwardsPostRequests(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/todos" {
			t.Errorf("unexpected backend request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"todo":"Created"}`)
	}))
	defer backend.Close()

	handler := todoProxyHandler(backend.Client(), backend.URL+"/todos")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(`{"todo":"Created"}`))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	if got, want := recorder.Body.String(), `{"todo":"Created"}`; strings.TrimSpace(got) != want {
		t.Fatalf("response body = %q, want %q", got, want)
	}
}

func TestImageCacheRefreshesAfterServingExpiredImageOnce(t *testing.T) {
	var fetches atomic.Int32
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetch := fetches.Add(1)
		fmt.Fprintf(w, "image-%d", fetch)
	}))
	defer imageServer.Close()

	now := time.Date(2026, time.November, 2, 0, 0, 0, 0, time.UTC)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	cache := newImageCache(cacheDir, imageServer.URL)
	cache.now = func() time.Time { return now }

	image, stale, err := cache.get()
	if err != nil || stale || string(image) != "image-1" {
		t.Fatalf("initial get = (%q, %t, %v), want image-1, fresh, nil", image, stale, err)
	}

	now = now.Add(imageTTL)
	cacheAfterRestart := newImageCache(cacheDir, imageServer.URL)
	cacheAfterRestart.now = func() time.Time { return now }
	image, stale, err = cacheAfterRestart.get()
	if err != nil || !stale || string(image) != "image-1" {
		t.Fatalf("first expired get = (%q, %t, %v), want image-1, stale, nil", image, stale, err)
	}

	image, stale, err = cacheAfterRestart.get()
	if err != nil || stale || string(image) != "image-2" {
		t.Fatalf("second expired get = (%q, %t, %v), want image-2, fresh, nil", image, stale, err)
	}
	if got := fetches.Load(); got != 2 {
		t.Fatalf("image fetch count = %d, want 2", got)
	}
}
