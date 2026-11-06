package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	db, err := openDatabase()
	if err != nil {
		log.Fatalf("open counter database: %v", err)
	}
	defer db.Close()

	store := &postgresCounterStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		log.Fatalf("initialize counter database: %v", err)
	}

	log.Println("Ping-pong listening on :8080")
	if err := http.ListenAndServe(":8080", newHandler(store)); err != nil {
		log.Fatalf("serve Ping-pong: %v", err)
	}
}

type counterStore interface {
	increment(context.Context) (uint64, error)
	count(context.Context) (uint64, error)
}

type postgresCounterStore struct {
	db *sql.DB
}

func (s *postgresCounterStore) initialize(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS ping_pong_counter (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			count BIGINT NOT NULL CHECK (count >= 0)
		);
	`)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO ping_pong_counter (id, count)
		VALUES (1, 0)
		ON CONFLICT (id) DO NOTHING;
	`)
	return err
}

func (s *postgresCounterStore) increment(ctx context.Context) (uint64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `
		UPDATE ping_pong_counter
		SET count = count + 1
		WHERE id = 1
		RETURNING count;
	`).Scan(&count)
	return uint64(count), err
}

func (s *postgresCounterStore) count(ctx context.Context) (uint64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `
		SELECT count FROM ping_pong_counter WHERE id = 1;
	`).Scan(&count)
	return uint64(count), err
}

func newHandler(store counterStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/pingpong", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		count, err := store.increment(r.Context())
		if err != nil {
			log.Printf("increment Ping-pong counter: %v", err)
			http.Error(w, "could not increment Ping-pong counter", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "pong %d\n", count)
	})
	mux.HandleFunc("/pings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		count, err := store.count(r.Context())
		if err != nil {
			log.Printf("read Ping-pong counter: %v", err)
			http.Error(w, "could not read Ping-pong counter", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "%d\n", count)
	})
	return mux
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
	if _, err := strconv.Atoi(port); err != nil {
		return nil, fmt.Errorf("invalid POSTGRES_PORT %q: %w", port, err)
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
