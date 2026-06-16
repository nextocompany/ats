// k6 load test for the CV intake path. Drives concurrent bulk uploads and asserts
// API latency + error-rate thresholds. The async pipeline (OCR+LLM) is the real
// capacity constraint — see README.md for the pipeline-drain measurement that
// complements this HTTP test.
//
// Run (against a STAGING stack — never prod data):
//   k6 run \
//     -e TARGET=https://<staging-api> \
//     -e POSITION_ID=<uuid> \
//     -e COOKIE="hr_auth=<session>" \
//     -e SAMPLE=./loadtest/sample.pdf \
//     loadtest/intake-load.js
//
// SAMPLE must be a real CV file (pdf/docx/png/jpg) you provide locally; it is NOT
// committed. COOKIE is an authenticated HR session (or set AUTH bearer below).
import http from "k6/http";
import { check, sleep } from "k6";

const TARGET = __ENV.TARGET || "http://localhost:8080";
const POSITION_ID = __ENV.POSITION_ID || "";
const COOKIE = __ENV.COOKIE || "";
const BEARER = __ENV.BEARER || "";
const SAMPLE = __ENV.SAMPLE || "./loadtest/sample.pdf";

// Load the sample CV once per VU init (binary).
const cvBin = open(SAMPLE, "b");
const cvName = SAMPLE.split("/").pop();

export const options = {
  scenarios: {
    ramp_uploads: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "30s", target: 10 },
        { duration: "1m", target: 30 },
        { duration: "30s", target: 0 },
      ],
      gracefulStop: "30s",
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"], // < 1% errors
    http_req_duration: ["p(95)<2000"], // API p95 < 2s (upload + enqueue)
  },
};

export default function () {
  if (!POSITION_ID) {
    throw new Error("POSITION_ID env is required (a valid position uuid)");
  }
  const headers = {};
  if (COOKIE) headers["Cookie"] = COOKIE;
  if (BEARER) headers["Authorization"] = `Bearer ${BEARER}`;

  const body = {
    position_id: POSITION_ID,
    source_channel: "loadtest",
    resumes: http.file(cvBin, cvName, "application/pdf"),
  };

  const res = http.post(`${TARGET}/api/v1/applications/bulk-intake`, body, { headers });
  check(res, {
    "status 200": (r) => r.status === 200,
    "envelope success": (r) => {
      try {
        return JSON.parse(r.body).success === true;
      } catch {
        return false;
      }
    },
  });
  sleep(1);
}
