package database

import (
	"context"
	"os"
	"testing"
)

func TestConnectUnreachable(t *testing.T) {
	// Port 1 on localhost refuses connections immediately.
	pool, err := Connect(context.Background(), "postgres://u:p@127.0.0.1:1/db?sslmode=disable")
	if err == nil {
		pool.Close()
		t.Fatal("Connect() error = nil, want error for unreachable database")
	}
}

func TestConnectIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	pool, err := Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}
