package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTodosCanBeFetchedAndCreated(t *testing.T) {
	handler := newTodoHandler()

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
		{name: "too long", body: `{"todo":"` + strings.Repeat("a", 141) + `"}`},
		{name: "malformed JSON", body: `{"todo":`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			newTodoHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(test.body)))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
		})
	}
}
