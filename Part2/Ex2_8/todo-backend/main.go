package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type todoRequest struct {
	Todo string `json:"todo"`
}

type todoStore interface {
	list(context.Context) ([]string, error)
	create(context.Context, string) error
}

type postgresTodoStore struct {
	db *sql.DB
}

func (s *postgresTodoStore) initialize(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin todo database initialization: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS todos (
			id BIGSERIAL PRIMARY KEY,
			todo TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("create todos table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS todo_seed (
			id INTEGER PRIMARY KEY CHECK (id = 1)
		)
	`); err != nil {
		return fmt.Errorf("create todo seed marker: %w", err)
	}

	result, err := tx.ExecContext(ctx, `INSERT INTO todo_seed (id) VALUES (1) ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("initialize todo seed marker: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check todo seed marker: %w", err)
	}
	if inserted == 1 {
		for _, todo := range []string{
			"Read the Kubernetes documentation",
			"Build a useful todo app",
			"Take a short break",
		} {
			if _, err := tx.ExecContext(ctx, `INSERT INTO todos (todo) VALUES ($1)`, todo); err != nil {
				return fmt.Errorf("seed initial todos: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit todo database initialization: %w", err)
	}
	return nil
}

func (s *postgresTodoStore) list(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT todo FROM todos ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	todos := make([]string, 0)
	for rows.Next() {
		var todo string
		if err := rows.Scan(&todo); err != nil {
			return nil, err
		}
		todos = append(todos, todo)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return todos, nil
}

func (s *postgresTodoStore) create(ctx context.Context, todo string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO todos (todo) VALUES ($1)`, todo)
	return err
}

func newTodoHandler(store todoStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/todos", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/todos" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			todos, err := store.list(r.Context())
			if err != nil {
				log.Printf("list todos: %v", err)
				http.Error(w, "could not read todos", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(todos); err != nil {
				log.Printf("encode todos: %v", err)
			}
		case http.MethodPost:
			r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
			var request todoRequest
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&request); err != nil {
				var maxBytesError *http.MaxBytesError
				if errors.As(err, &maxBytesError) {
					http.Error(w, "request body is too large", http.StatusRequestEntityTooLarge)
					return
				}
				http.Error(w, "request body must be valid JSON with a todo field", http.StatusBadRequest)
				return
			}
			var extra any
			if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
				http.Error(w, "request body must contain one JSON object", http.StatusBadRequest)
				return
			}
			todo := strings.TrimSpace(request.Todo)
			if todo == "" {
				http.Error(w, "todo must not be empty", http.StatusBadRequest)
				return
			}
			if utf8.RuneCountInString(todo) > 140 {
				http.Error(w, "todo must be 140 characters or fewer", http.StatusBadRequest)
				return
			}
			if err := store.create(r.Context(), todo); err != nil {
				log.Printf("create todo: %v", err)
				http.Error(w, "could not save todo", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			if err := json.NewEncoder(w).Encode(todoRequest{Todo: todo}); err != nil {
				log.Printf("encode created todo: %v", err)
			}
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func main() {
	port, ok := os.LookupEnv("PORT")
	if !ok || port == "" {
		log.Fatal("PORT must be set")
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		log.Fatalf("invalid PORT %q: must be an integer from 1 to 65535", port)
	}

	db, err := openDatabase()
	if err != nil {
		log.Fatalf("open todo database: %v", err)
	}
	defer db.Close()

	store := &postgresTodoStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		log.Fatalf("initialize todo database: %v", err)
	}
	server := &http.Server{
		Addr:              net.JoinHostPort("", port),
		Handler:           newTodoHandler(store),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("Todo backend listening on port %s", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve todo backend: %v", err)
	}
}

func openDatabase() (*sql.DB, error) {
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	database := os.Getenv("POSTGRES_DB")
	if host == "" || port == "" || user == "" || password == "" || database == "" {
		return nil, fmt.Errorf("POSTGRES_HOST, POSTGRES_PORT, POSTGRES_USER, POSTGRES_PASSWORD and POSTGRES_DB must be set")
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		return nil, fmt.Errorf("invalid POSTGRES_PORT %q: must be an integer from 1 to 65535", port)
	}

	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
		Path:   database,
	}
	query := dsn.Query()
	query.Set("sslmode", "disable")
	dsn.RawQuery = query.Encode()

	db, err := sql.Open("pgx", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("configure PostgreSQL connection: %w", err)
	}
	db.SetMaxOpenConns(10)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var lastErr error
	for {
		if err := db.PingContext(ctx); err == nil {
			return db, nil
		} else {
			lastErr = err
		}
		if ctx.Err() != nil {
			db.Close()
			return nil, fmt.Errorf("wait for PostgreSQL: %w; last connection error: %v", ctx.Err(), lastErr)
		}
		time.Sleep(2 * time.Second)
	}
}
