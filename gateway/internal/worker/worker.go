package worker

import (
	"log"
	"time"
)

const (
	MaxAttempts     = 5
	MaxPollAttempts = 10
	DefaultInterval = 2 * time.Second
	PollInterval    = 3 * time.Second
)

// Job represents a single enrichment task pushed onto the channel.
type Job struct {
	EnrichmentID int64
	CaseID       string
}

// Processor is implemented by the enrichment service.
// Defined here to avoid an import cycle (enrichments imports worker.Job).
type Processor interface {
	ProcessEnrichment(enrichmentID int64, caseID string) error
}

// StartWorkerPool spins up numWorkers goroutines consuming from jobs.
// All enrichment logic is delegated to processor (the enrichment service).
func StartWorkerPool(processor Processor, jobs <-chan Job, numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		go runWorker(processor, jobs)
	}
}

func runWorker(processor Processor, jobs <-chan Job) {
	for job := range jobs {
		log.Printf("worker: picked up job enrichmentID=%d caseID=%s", job.EnrichmentID, job.CaseID)
		if err := processor.ProcessEnrichment(job.EnrichmentID, job.CaseID); err != nil {
			log.Printf("worker: job failed enrichmentID=%d caseID=%s: %v", job.EnrichmentID, job.CaseID, err)
		} else {
			log.Printf("worker: job completed enrichmentID=%d caseID=%s", job.EnrichmentID, job.CaseID)
		}
	}
}
