package main

import (
	"log"
	"net/http"

	"github.com/zhalok/integration-gateway/internal/api"
	"github.com/zhalok/integration-gateway/internal/assets"
	"github.com/zhalok/integration-gateway/internal/db"
)

func main() {
	database, err := db.Connect()
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database, assets.Schema); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	if err := db.Seed(database, assets.CasesJSON); err != nil {
		log.Fatalf("seed: %v", err)
	}

	http.HandleFunc("/api/health", api.HealthHandler)

	log.Println("server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("server: %v", err)
	}
}
