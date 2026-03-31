# Backend Engineer — Take-Home Assignment

## Integration Gateway

---

## Context

Pearson Specter Litt is an AI-native SaaS platform for foreclosure law firms. These firms manage hundreds to thousands of active legal cases simultaneously, each requiring data from multiple external systems — property records databases, court filing systems, military status registries — that are unreliable, inconsistently formatted, and rate-limited.

The **Integration Gateway** is a core backend service responsible for enriching case records by orchestrating data retrieval from these external sources. Each source has different protocols, failure modes, and latency characteristics. The gateway must handle all of this gracefully — retrying transient failures, respecting rate limits, tracking partial results, and never losing or duplicating work.

Your assignment is to build a simplified Integration Gateway that demonstrates how you approach the core backend challenges of integrating with unreliable external systems in a production environment.

---

## What to Build

A Go service with a REST API that enriches foreclosure case records by fetching data from three simulated external services:

1. **Property Records API** — Returns property ownership, liens, and tax information. REST/JSON. Intermittently returns 503 errors and occasionally responds slowly.
2. **Court Records System** — Returns case filings, hearing schedules, and judge assignments. Accepts JSON requests but returns **XML** responses. Rate-limited to 2 requests/second.
3. **SCRA Military Status Check** — Checks if a borrower is on active military duty (legally required before foreclosure can proceed). **Asynchronous:** you submit a search request, receive a search ID, then poll for results.

Each service has realistic failure modes. A mock server simulating all three is provided — you run it locally and integrate against it.

---

## Technical Stack

- **Required:** Go 1.21+, PostgreSQL
- **Your choice:** Any Go libraries for HTTP, database access, XML parsing, etc. We evaluate architecture, not dependency choices.
- **No frontend needed.** This is a pure backend service with a REST API.

---

## Mock Services

A Go mock server is provided in `backend_assignment_data/mock_server/`. Run it:

```bash
cd backend_assignment_data/mock_server
go run main.go
```

This starts three services:

| Service | Port | Protocol | Failure Behavior |
|---|---|---|---|
| Property Records | 9001 | REST/JSON | ~25% of requests return 503. ~10% respond after 8-second delay. |
| Court Records | 9002 | JSON request → XML response | Rate-limited: 2 req/sec (429 with `Retry-After` header). ~3% return malformed XML. |
| SCRA Check | 9003 | REST/JSON, async | Submit search → 202. Poll for result → 200 when ready (2-5 sec). ~10% of searches fail permanently. |

Failure rates are configurable via environment variables (see mock server README). The defaults above are the evaluation settings.

**Do not modify the mock server.** Your service must integrate with it as-is, handling all failure modes.

---

## Mock Service API Contracts

### Property Records — `GET http://localhost:9001/api/properties/{state}/{county}/{parcelId}`

Returns property ownership, liens, and tax status.

**Success (200):**
```json
{
  "parcelId": "14-28-322-001-0000",
  "county": "Cook",
  "state": "IL",
  "address": "1422 W Diversey Pkwy, Chicago, IL 60614",
  "owner": {
    "name": "Elena Martinez",
    "vestingType": "Fee Simple",
    "deedDate": "2019-06-15",
    "deedType": "Warranty Deed",
    "instrument": "2019R0567890"
  },
  "liens": [
    {
      "position": 1,
      "type": "Mortgage",
      "holder": "JPMorgan Chase Bank, N.A.",
      "amount": 385000.00,
      "recordedDate": "2019-06-15",
      "instrument": "2019R0567891",
      "status": "Active"
    }
  ],
  "taxStatus": {
    "year": 2025,
    "status": "Current",
    "amount": 6240.00,
    "parcelNumber": "14-28-322-001-0000"
  },
  "legalDescription": "LOT 14 IN BLOCK 3 OF SHEFFIELD'S ADDITION TO CHICAGO...",
  "easements": ["ComEd utility easement (Book 12045, Page 223)"],
  "lastUpdated": "2026-03-15T14:30:00Z"
}
```

**Not Found (404):** Property not in database.
**Service Unavailable (503):** Simulated transient failure. Retry.
**Slow Response:** Some successful responses take ~8 seconds.

---

### Court Records — `POST http://localhost:9002/api/court-records/search`

Accepts a JSON body, returns an **XML** response with court filings.

**Request:**
```json
{
  "caseNumber": "2026-CA-003891"
}
```

**Success (200):**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<CourtRecordResponse>
  <CaseNumber>2026-CA-003891</CaseNumber>
  <Court>Circuit Court of Miami-Dade County</Court>
  <Division>Civil</Division>
  <Judge>Hon. Patricia Navarro</Judge>
  <FilingDate>2025-11-20</FilingDate>
  <CaseType>Foreclosure</CaseType>
  <Status>Active</Status>
  <Parties>
    <Plaintiff>Wells Fargo Bank, N.A.</Plaintiff>
    <Defendant>David R. Thompson</Defendant>
  </Parties>
  <Filings>
    <Filing>
      <Type>Complaint</Type>
      <FiledDate>2025-11-20</FiledDate>
      <DocumentNumber>2025-CI-28934</DocumentNumber>
    </Filing>
  </Filings>
  <NextHearing>
    <Date>2026-04-22</Date>
    <Time>10:00</Time>
    <Type>Case Management Conference</Type>
    <Courtroom>5-3</Courtroom>
  </NextHearing>
</CourtRecordResponse>
```

**Not Found (200 with status):**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<CourtRecordResponse>
  <CaseNumber>REQUESTED-NUMBER</CaseNumber>
  <Status>NoFilingFound</Status>
  <Message>No court filings found for the specified case number.</Message>
</CourtRecordResponse>
```

**Rate Limited (429):** Includes `Retry-After` header (in seconds). Do not exceed 2 requests/second.
**Malformed XML (~3%):** Response is truncated mid-document. Handle parsing errors gracefully.

---

### SCRA Military Status — `POST http://localhost:9003/api/scra/search`

**Step 1: Submit search**

```json
{
  "lastName": "Martinez",
  "firstName": "Elena",
  "ssnLast4": "4521",
  "dob": "1985-03-14"
}
```

**Response (202 Accepted):**
```json
{
  "searchId": "scra-1711800000123456",
  "status": "pending",
  "submittedAt": "2026-03-16T10:00:00Z",
  "estimatedCompletionSeconds": 3
}
```

**Step 2: Poll for results — `GET http://localhost:9003/api/scra/results/{searchId}`**

**Still pending (202):**
```json
{
  "searchId": "scra-1711800000123456",
  "status": "pending"
}
```

**Complete (200):**
```json
{
  "searchId": "scra-1711800000123456",
  "status": "complete",
  "result": {
    "activeDuty": false,
    "lastName": "Martinez",
    "firstName": "Elena",
    "searchDate": "2026-03-16",
    "certificateUrl": null
  },
  "completedAt": "2026-03-16T10:00:03Z"
}
```

**Failed (200 with error status):**
```json
{
  "searchId": "scra-1711800000123456",
  "status": "error",
  "error": "Search failed: unable to verify military status"
}
```

**Not Found (404):** Invalid search ID.

---

## Case Data

> **See:** `backend_assignment_data/cases.json`

Six foreclosure cases to enrich. Load these into your PostgreSQL as seed data. Each case includes the information needed to query the three external services.

```go
type Case struct {
    ID              string   `json:"id"`
    CaseNumber      string   `json:"caseNumber"`
    Borrower        Borrower `json:"borrower"`
    Property        Property `json:"property"`
    Loan            Loan     `json:"loan"`
    CurrentStage    string   `json:"currentStage"`
    CourtCaseNumber *string  `json:"courtCaseNumber"` // null if pre-filing
}

type Borrower struct {
    FirstName string `json:"firstName"`
    LastName  string `json:"lastName"`
    SSNLast4  string `json:"ssnLast4"`
    DOB       string `json:"dob"`
}

type Property struct {
    Address  string `json:"address"`
    County   string `json:"county"`
    State    string `json:"state"`
    ParcelID string `json:"parcelId"`
}

type Loan struct {
    Number         string  `json:"number"`
    Servicer       string  `json:"servicer"`
    OriginalAmount float64 `json:"originalAmount"`
}
```

**How case fields map to external service queries:**

| Service | Lookup Fields | Which cases have data |
|---|---|---|
| Property Records | `property.state`, `property.county`, `property.parcelId` → URL path | Cases 1-5 return data. Case 6 returns 404 (property not in database). |
| Court Records | `courtCaseNumber` → POST body | Cases 2, 3, 4, 6 have court case numbers. Cases 1, 5 are `null` — skip. |
| SCRA | `borrower.lastName`, `borrower.firstName`, `borrower.ssnLast4`, `borrower.dob` → POST body | All 6 cases have SCRA data. Case 4 returns active military duty. |

**Important:** Cases at the `title-search` stage have no court case number yet (the case hasn't been filed). Your service should recognize this and skip the Court Records enrichment for those cases — marking that source as `not_applicable` rather than `failed`.

**Non-retryable errors:** A `404` from Property Records means the property is not in the database — this is a permanent condition, not a transient failure. Do not retry 404s. Only retry transient errors (503, 429).

**What the 6 cases test:**

| Case | Borrower | State | Stage | Key Challenge |
|---|---|---|---|---|
| 1 | Martinez, Elena | IL | title-search | No court case number — skip court records. 2 liens. |
| 2 | Thompson, David R. | FL | filing | 3 liens including HOA lis pendens. Court records available. |
| 3 | Nguyen, Thanh | TX | schedule-sale | Clean property, 1 lien. Full enrichment. |
| 4 | Johnson, Marcus | NY | serve-borrower | **SCRA returns active military duty.** 2 liens. |
| 5 | Williams, Sarah | OH | title-search | Delinquent taxes, tax sale pending. No court records. |
| 6 | Patel, Vikram | TX | serve-borrower | **Property records returns 404** — property not in database. |

---

## Requirements

### Must Have

These are the core requirements. All must be implemented.

**1. Case Enrichment API**

Your service exposes these endpoints:

- `POST /api/cases/{id}/enrich` — Trigger enrichment for a case. Fetches data from all applicable external services. Returns `202 Accepted` immediately (enrichment happens asynchronously). **Idempotent** — calling twice does not duplicate work or create inconsistent state.
- `GET /api/cases/{id}/enrichment` — Return the current enrichment status and data for a case, including per-source status.
- `GET /api/cases` — List all cases with their enrichment status summary.

**2. Per-Source Status Tracking**

The enrichment response must track each source independently:

```json
{
  "caseId": "case-001",
  "status": "partial",
  "sources": {
    "propertyRecords": {
      "status": "success",
      "attempts": 3,
      "lastAttempt": "2026-03-16T10:01:12Z",
      "data": { "...property data..." }
    },
    "courtRecords": {
      "status": "not_applicable",
      "reason": "Case is pre-filing — no court case number"
    },
    "scra": {
      "status": "pending",
      "attempts": 1,
      "lastAttempt": "2026-03-16T10:01:14Z",
      "searchId": "scra-1711800000123456"
    }
  },
  "startedAt": "2026-03-16T10:01:10Z",
  "completedAt": null
}
```

Valid source statuses: `pending`, `success`, `failed`, `not_applicable`.
Overall enrichment status: `pending` (in progress), `complete` (all applicable sources succeeded), `partial` (some succeeded, some failed — recoverable), `failed` (all sources failed).

**3. Resilience Patterns**

- **Retry with backoff:** Transient failures (503 from Property Records, 429 from Court Records) must be retried with exponential backoff and jitter. Do not retry indefinitely — cap at a reasonable number of attempts.
- **Rate limit compliance:** Respect the Court Records rate limit. When you receive a 429 with `Retry-After`, wait the specified duration before retrying.
- **Circuit breaker:** If a service is returning errors consistently, stop hitting it temporarily. The circuit should open after repeated failures and close after a cooldown period. Circuit breaker state should be visible via the health endpoint.
- **Timeouts:** Set reasonable timeouts for each external call. The Property Records service occasionally takes 8+ seconds — your timeout policy should account for this without blocking forever.

**4. Async Polling**

The SCRA service requires a submit-then-poll pattern:
1. Submit the search request, receive a search ID.
2. Poll the results endpoint until the search completes (or fails).
3. Use reasonable polling intervals (not tight loops).
4. Handle the case where the search fails permanently after polling.

**5. Data Layer**

- Store case data and enrichment results in **PostgreSQL**.
- Design a schema that models: cases, per-source enrichment state, enrichment data, and attempt history.
- Include your schema as migration files or a `schema.sql` in the repo.
- The schema should cleanly handle: partial enrichment (some sources done, others pending), multiple attempts per source, and the different data shapes from each source.

**6. Idempotency**

Calling `POST /api/cases/{id}/enrich` multiple times must not:
- Duplicate enrichment attempts for sources that already succeeded.
- Create duplicate database records.
- Re-trigger work that is already in progress.

If enrichment is already complete, return the existing result. If enrichment is in progress, return the current state. If a previous attempt partially failed, re-trigger only the failed sources.

**7. Health Endpoint**

`GET /api/health` returns service health including circuit breaker states:

```json
{
  "status": "healthy",
  "circuitBreakers": {
    "propertyRecords": { "state": "closed", "failures": 2, "lastFailure": "..." },
    "courtRecords": { "state": "open", "openedAt": "...", "cooldownEnds": "..." },
    "scra": { "state": "closed", "failures": 0 }
  }
}
```

---

### Nice to Have

Pick **any** of these that interest you. These are not required, but they let you demonstrate depth in areas you're strongest in. We'd rather see one done well than three done superficially.

- **Workflow Orchestration:** Use Temporal, Cadence, or a similar durable workflow framework to orchestrate the enrichment process. Define the enrichment as a workflow with activities for each source, leveraging the framework's built-in retry, timeout, and state persistence capabilities.

- **Concurrent Enrichment:** When enriching a single case, fetch from independent sources concurrently (Property Records and SCRA can run in parallel; Court Records depends on having a court case number). When enriching multiple cases, demonstrate controlled concurrency (e.g., a worker pool that respects the Court Records rate limit globally).

- **Bulk Enrichment:** Add `POST /api/enrich/bulk` — accept a list of case IDs and enrich them with controlled concurrency. Report progress per case.

- **Metrics & Observability:** Expose Prometheus-compatible metrics: request latency per external service, error rates, circuit breaker state transitions, enrichment completion rates. Structured logging with correlation IDs that trace an enrichment request across all external calls.

- **Webhook / Callback:** Instead of polling the enrichment status, support an optional `callbackUrl` parameter on the enrich endpoint that receives a POST when enrichment completes.

- **Containerized Deployment:** Provide a `Dockerfile` for your service and a `docker-compose.yml` that starts your service, PostgreSQL, and the mock server together with a single command.

---

## Constraints

- **Time budget:** We expect this to take **6-8 hours** of focused work. Do not over-polish. We would rather see clean architecture with solid resilience patterns than a feature-complete service with naive error handling.
- **Mock services are your external world.** Treat them as you would treat real, unreliable, third-party systems. Do not hardcode assumptions about their response data — parse and validate what you receive.
- **PostgreSQL is required.** A `docker-compose.yml` for PostgreSQL is included for convenience (`backend_assignment_data/docker-compose.yml`), but you may use any local PostgreSQL instance.

---

## Deliverables

1. **A GitHub repository** (public or private with access granted) containing the working service with a `README.md` that includes:
   - Instructions to run the service (including database setup)
   - How to trigger enrichment and observe results
   - Brief description of the architecture

2. **`APPROACH.md`** (maximum 2 pages) covering:
   - Your key architectural decisions — how you structured the service, why you chose your concurrency model, how you designed the schema.
   - Your resilience strategy — how retry, circuit breaker, and timeout policies interact. What happens when everything goes wrong simultaneously.
   - What you would change, refactor, or add if you had more time.
   - Which "Nice to Have" items you chose and why.

3. **Database schema** — as migration files or `schema.sql`, included in the repo.

---

## Evaluation Criteria

| Area | Weight | What We Evaluate |
|---|---|---|
| **Integration Resilience** | 25% | Retry with backoff, circuit breakers, rate limit compliance, timeout handling. Does the service degrade gracefully under failure? Does it recover when services come back? |
| **Go & API Design** | 25% | Idiomatic Go. Clean error handling. Well-structured HTTP handlers. Proper use of interfaces, goroutines, and channels. API design is consistent and RESTful. |
| **Data Layer & Schema** | 20% | PostgreSQL schema models the enrichment domain cleanly. Handles partial states, multiple attempts, and different source data shapes. Migration discipline. |
| **Async & Concurrency** | 15% | SCRA polling is handled correctly. Concurrent enrichment doesn't create race conditions. Idempotency holds under concurrent requests. |
| **Observability & Operations** | 10% | Structured logging. Health endpoint with circuit breaker state. Errors are actionable, not generic. An operator could diagnose issues from the logs. |
| **Code Quality & Documentation** | 5% | Readable, well-organized code. Clear README and APPROACH.md. No dead code, no over-engineering. |

---

## Why This Assignment

This is a scaled-down slice of the actual **Integration Hub** you would be building at Pearson Specter Litt. The production system integrates with Black Knight (SOAP/XML, $3,000 per API endpoint), county court e-filing systems (each with unique protocols), the DoD SCRA database (rate-limited, async), PACER (the federal court records system), title company APIs, and dozens of county-specific services.

Every integration is unreliable. Every integration has different failure modes. Every integration costs money per call. The backend engineer who owns this layer must build systems that never lose data, never duplicate expensive calls, and give operators clear visibility into what's working and what isn't.

We want to see how you approach that problem — not just "call three APIs," but build a system that handles the reality of external dependencies in a production environment.

Good luck. We look forward to reviewing your work.
