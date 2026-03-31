CREATE TABLE IF NOT EXISTS cases (
    id                TEXT PRIMARY KEY,
    case_number       TEXT NOT NULL,
    current_stage     TEXT NOT NULL,
    court_case_number TEXT,

    -- borrower
    borrower_first_name TEXT NOT NULL,
    borrower_last_name  TEXT NOT NULL,
    borrower_ssn_last4  TEXT NOT NULL,
    borrower_dob        TEXT NOT NULL,

    -- property
    property_address   TEXT NOT NULL,
    property_county    TEXT NOT NULL,
    property_state     TEXT NOT NULL,
    property_parcel_id TEXT NOT NULL,

    -- loan
    loan_number          TEXT NOT NULL,
    loan_servicer        TEXT NOT NULL,
    loan_original_amount NUMERIC NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS enrichments (
    id           BIGSERIAL PRIMARY KEY,
    case_id      TEXT NOT NULL REFERENCES cases(id) UNIQUE,
    status       TEXT NOT NULL DEFAULT 'pending', -- pending, complete, partial, failed
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,

    -- property records
    pr_status       TEXT NOT NULL DEFAULT 'pending', -- pending, success, failed, not_applicable
    pr_attempts     INT NOT NULL DEFAULT 0,
    pr_last_attempt TIMESTAMPTZ,
    pr_retry_after  TIMESTAMPTZ,
    pr_data         JSONB,
    pr_reason       TEXT,

    -- court records
    cr_status       TEXT NOT NULL DEFAULT 'pending',
    cr_attempts     INT NOT NULL DEFAULT 0,
    cr_last_attempt TIMESTAMPTZ,
    cr_retry_after  TIMESTAMPTZ,
    cr_data         JSONB,
    cr_reason       TEXT,

    -- scra
    scra_status       TEXT NOT NULL DEFAULT 'pending',
    scra_attempts     INT NOT NULL DEFAULT 0,
    scra_last_attempt TIMESTAMPTZ,
    scra_retry_after  TIMESTAMPTZ,
    scra_search_id    TEXT,
    scra_data         JSONB,
    scra_reason       TEXT
);
