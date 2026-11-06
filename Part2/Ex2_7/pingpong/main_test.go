package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type memoryCounter struct {
	mu    sync.Mutex
	value uint64
	err   error
}

func (c *memoryCounter) increment(context.Context) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return 0, c.err
	}
	c.value++
	return c.value, nil
}

func (c *memoryCounter) count(context.Context) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value, c.err
}

func TestPingPongAndPingsEndpoints(t *testing.T) {
	counter := &memoryCounter{}
	handler := newHandler(counter)
	for i := 1; i <= 3; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/pingpong", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("pingpong status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if got, want := recorder.Body.String(), fmt.Sprintf("pong %d\n", i); got != want {
			t.Fatalf("pingpong response = %q, want %q", got, want)
		}
	}

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/pings", nil))
		if got, want := recorder.Body.String(), "3\n"; got != want {
			t.Fatalf("pings response = %q, want %q", got, want)
		}
	}
}

func TestCounterEndpointsReportStoreErrors(t *testing.T) {
	handler := newHandler(&memoryCounter{err: errors.New("database unavailable")})
	for _, path := range []string{"/pingpong", "/pings"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("%s status = %d, want %d", path, recorder.Code, http.StatusInternalServerError)
		}
	}
}

func TestCounterEndpointsRejectNonGetMethods(t *testing.T) {
	handler := newHandler(&memoryCounter{})
	for _, path := range []string{"/pingpong", "/pings"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, nil))
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s status = %d, want %d", path, recorder.Code, http.StatusMethodNotAllowed)
		}
	}
}
