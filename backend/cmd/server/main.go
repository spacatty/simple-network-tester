package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"loadtester/backend/internal/api"
	"loadtester/backend/internal/proxy"
	"loadtester/backend/internal/runner"
	"loadtester/backend/internal/storage"
)

func main() {
	port := envOr("PORT", "8080")
	dataDir := envOr("DATA_DIR", "./data")
	authToken := os.Getenv("AUTH_TOKEN")
	if authToken == "" {
		log.Fatal("AUTH_TOKEN is required")
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "uploads"), 0o755); err != nil {
		log.Fatalf("failed to create uploads dir: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}

	store, err := storage.New(filepath.Join(dataDir, "loadtester.db"))
	if err != nil {
		log.Fatalf("failed to init storage: %v", err)
	}
	proxyManager := proxy.NewManager(store)
	runService := runner.NewService(store, proxyManager)
	server := api.NewServer(store, runService, proxyManager, filepath.Join(dataDir, "uploads"), authToken)

	log.Printf("backend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, server.Routes()); err != nil {
		log.Fatal(err)
	}
}

func envOr(name, fallback string) string {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	return v
}
