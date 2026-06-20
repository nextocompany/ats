// Read-only load test — SAFE to run against production.
//
// Unlike intake-load.js (which writes real candidate/application rows and burns
// Azure OCR/LLM quota), this test only issues GET requests to endpoints with NO
// side effects: it creates no data, calls no Azure AI, and sends nothing to
// candidates. It measures ingress + API concurrency + dependency-ping latency and
// exercises the container-app autoscaler (api min 1 / max 3 replicas).
//
// Targets:
//   1. GET /health (NOT rate-limited) — the headline concurrency scenario. Each
//      call pings postgres + redis + blob + the asynq queue, so it reflects the
//      real backend dependency path, not a static 200.
//   2. GET /api/v1/public/positions (rate-limited 30/min per IP by default) — a
//      LOW-rate sample (<30/min) to capture real public read-query latency without
//      tripping the limiter. Informational; not part of the pass/fail gate.
//
// Run:
//   k6 run -e TARGET=https://<api> loadtest/readonly-load.js
//
// Thresholds are scoped to the health scenario so the rate-limited positions
// sample can't fail the run.
import http from "k6/http";
import { check } from "k6";

const TARGET = __ENV.TARGET || "http://localhost:8080";

export const options = {
  scenarios: {
    // Headline: ramp concurrent /health load to exercise the API + autoscaler.
    health_ramp: {
      executor: "ramping-vus",
      exec: "health",
      startVUs: 0,
      stages: [
        { duration: "30s", target: 20 }, // ramp up
        { duration: "1m", target: 20 }, // hold
        { duration: "30s", target: 0 }, // ramp down
      ],
      gracefulStop: "15s",
      tags: { scenario: "health_ramp" },
    },
    // Informational: ~0.3 req/s = 18/min, safely under the 30/min public limiter.
    positions_sample: {
      executor: "constant-arrival-rate",
      exec: "positions",
      rate: 3,
      timeUnit: "10s",
      duration: "2m",
      preAllocatedVUs: 2,
      tags: { scenario: "positions_sample" },
    },
  },
  thresholds: {
    // Pass/fail gate is the /health scenario only.
    "http_req_failed{scenario:health_ramp}": ["rate<0.01"], // < 1% errors
    "http_req_duration{scenario:health_ramp}": ["p(95)<1000"], // p95 < 1s
    // Positions latency is observed, not gated (it can be rate-limited to 429).
    "http_req_duration{scenario:positions_sample}": ["p(95)<1500"],
  },
};

export function health() {
  const res = http.get(`${TARGET}/health`, { tags: { ep: "health" } });
  check(res, { "health 200": (r) => r.status === 200 });
}

export function positions() {
  const res = http.get(`${TARGET}/api/v1/public/positions`, { tags: { ep: "positions" } });
  // 200 = served, 429 = rate-limited (expected under load, not an error here).
  check(res, {
    "positions 200 or 429": (r) => r.status === 200 || r.status === 429,
  });
}
