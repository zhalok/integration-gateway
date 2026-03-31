package main

import (
	"log"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/zhalok/integration-gateway/internal/api"
	"github.com/zhalok/integration-gateway/internal/assets"
	"github.com/zhalok/integration-gateway/internal/circuitbreaker"
	"github.com/zhalok/integration-gateway/internal/clients"
	"github.com/zhalok/integration-gateway/internal/db"
	"github.com/zhalok/integration-gateway/internal/domains/cases"
	"github.com/zhalok/integration-gateway/internal/domains/enrichments"
	"github.com/zhalok/integration-gateway/internal/worker"
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

	// --- infrastructure ---
	cbs := circuitbreaker.NewSet()

	// Court Records is rate-limited to 2 req/sec globally across all workers
	courtRateLimiter := rate.NewLimiter(rate.Every(500*time.Millisecond), 1)

	jobs := make(chan worker.Job, 100)

	// --- clients ---
	propertyClient := clients.NewPropertyClient()
	courtClient := clients.NewCourtClient(courtRateLimiter)
	scraClient := clients.NewSCRAClient()

	// --- cases domain ---
	caseRepo := cases.NewRepository(database)
	caseSvc := cases.NewService(caseRepo)
	caseUsecase := cases.NewUsecase(caseSvc)
	caseController := cases.NewController(caseUsecase)

	// --- enrichments domain ---
	enrichmentRepo := enrichments.NewRepository(database)
	enrichmentSvc := enrichments.NewService(enrichmentRepo, caseRepo, propertyClient, courtClient, scraClient, cbs, jobs)
	enrichmentUsecase := enrichments.NewUsecase(enrichmentSvc, caseSvc, jobs)
	enrichmentController := enrichments.NewController(enrichmentUsecase)

	// --- worker pool ---
	go worker.StartWorkerPool(enrichmentSvc, jobs, 5)

	// --- routes ---
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", api.HealthHandler(cbs))
	caseController.RegisterRoutes(mux)
	enrichmentController.RegisterRoutes(mux)

	log.Println("server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}
