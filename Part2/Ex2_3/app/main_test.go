package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogOutputFetchesPingPongCount(t *testing.T) {
	pingPong := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/pings" {
			t.Errorf("unexpected Ping-pong request: %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte("3\n"))
	}))
	defer pingPong.Close()

	output, err := newLogOutput(pingPong.URL + "/pings")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	output.serveHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), output.latestLine()+"\nPing / Pongs: 3\n") {
		t.Fatalf("unexpected response: %q", recorder.Body.String())
	}
}

func TestLogOutputReportsPingPongUnavailable(t *testing.T) {
	output, err := newLogOutput("http://127.0.0.1:1/pings")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	output.serveHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}
