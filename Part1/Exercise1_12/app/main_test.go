package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

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
