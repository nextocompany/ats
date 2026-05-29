-- Sprint 5b: persisted recurring/on-demand report exports. Blob columns hold the
-- stored object URLs; delivered tracks whether the notification link was sent.
-- The (kind, period) unique index makes export generation idempotent under asynq
-- retries (the handler upserts rather than inserting duplicates).

CREATE TABLE report_exports (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind       VARCHAR(20) NOT NULL,
    period     VARCHAR(40) NOT NULL,
    csv_blob   TEXT,
    json_blob  TEXT,
    delivered  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (kind, period)
);

CREATE INDEX idx_report_exports_created ON report_exports (created_at DESC);
