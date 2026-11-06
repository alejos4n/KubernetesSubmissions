package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type memoryTodoStore struct {
	mu    sync.Mutex
	todos []string
	err   error
}

func (s *memoryTodoStore) list(context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.todos...), s.err
}

func (s *memoryTodoStore) create(_ context.Context, todo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.todos = append(s.todos, todo)
	return nil
}

func TestTodosCanBeFetchedAndCreated(t *testing.T) {
	store := &memoryTodoStore{todos: []string{
		"Read the Kubernetes documentation",
		"Build a useful todo app",
		"Take a short break",
	}}
	handler := newTodoHandler(store)

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/todos", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", get.Code, http.StatusOK)
	}
	var todos []string
	if err := json.Unmarshal(get.Body.Bytes(), &todos); err != nil {
		t.Fatal(err)
	}
	if len(todos) != 3 {
		t.Fatalf("initial todo count = %d, want 3", len(todos))
	}

	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(`{"todo":"  Ship it  "}`)))
	if post.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want %d: %s", post.Code, http.StatusCreated, post.Body.String())
	}

	get = httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/todos", nil))
	if err := json.Unmarshal(get.Body.Bytes(), &todos); err != nil {
		t.Fatal(err)
	}
	if len(todos) != 4 || todos[3] != "Ship it" {
		t.Fatalf("todos after POST = %#v, want appended trimmed todo", todos)
	}
}

func TestCreateTodoValidatesLengthAndBody(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty", body: `{"todo":"  "}`},
		{name: "too long", body: `{"todo":"` + strings.Repeat("a", maxTodoLength+1) + `"}`},
		{name: "malformed JSON", body: `{"todo":`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			newTodoHandler(&memoryTodoStore{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(test.body)))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestTodoLengthLimitAllows140Characters(t *testing.T) {
	todo := strings.Repeat("a", maxTodoLength)
	store := &memoryTodoStore{}
	recorder := httptest.NewRecorder()
	newTodoHandler(store).ServeHTTP(recorder, httptest.NewRequest(
		http.MethodPost,
		"/todos",
		strings.NewReader(`{"todo":"`+todo+`"}`),
	))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if len(store.todos) != 1 || store.todos[0] != todo {
		t.Fatalf("saved todos = %#v, want the 140-character todo", store.todos)
	}
}

func TestTooLongTodoIsRejectedAndLogged(t *testing.T) {
	previousOutput := log.Writer()
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() {
		log.SetOutput(previousOutput)
	})

	todo := strings.Repeat("x", maxTodoLength+1)
	store := &memoryTodoStore{}
	recorder := httptest.NewRecorder()
	newTodoHandler(store).ServeHTTP(recorder, httptest.NewRequest(
		http.MethodPost,
		"/todos",
		strings.NewReader(`{"todo":"`+todo+`"}`),
	))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), "todo must be 140 characters or fewer") {
		t.Fatalf("response does not explain the limit: %q", recorder.Body.String())
	}
	if len(store.todos) != 0 {
		t.Fatalf("rejected todo was saved: %#v", store.todos)
	}
	if !strings.Contains(logs.String(), "todo rejected: reason=exceeds_limit characters=141 limit=140") ||
		!strings.Contains(logs.String(), todo) {
		t.Fatalf("rejected todo was not fully logged: %s", logs.String())
	}
}

func TestTodoDatabaseErrorsAreReported(t *testing.T) {
	handler := newTodoHandler(&memoryTodoStore{err: errors.New("database unavailable")})
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/todos", nil),
		httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(`{"todo":"Ship it"}`)),
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("%s status = %d, want %d", request.Method, recorder.Code, http.StatusInternalServerError)
		}
	}
}
