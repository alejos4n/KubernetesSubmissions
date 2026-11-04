package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPingPongAndPingsEndpoints(t *testing.T) {
	handler := newHandler()
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
