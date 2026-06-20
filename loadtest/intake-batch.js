// Bounded intake batch — concurrency + pipeline-drain measurement.
//
// Unlike intake-load.js (time-based ramp → unbounded CV count), this submits a
// FIXED total of CVs (CVS, default 100) at a fixed concurrency (VUS, default 10),
// so the async pipeline (OCR+LLM) is loaded by a known, bounded batch you can then
// watch drain. WRITES REAL ROWS — run only against a disposable stack, and clean up
// (source_channel='loadtest') after. See README + the deploy operator's cleanup.
//
// Run:
//   k6 run -e TARGET=https://<api> -e POSITION_ID=<uuid> -e COOKIE="hr_auth=…" \
//     -e SAMPLE=./loadtest/sample.pdf -e CVS=100 -e VUS=10 loadtest/intake-batch.js
import http from "k6/http";
import { check } from "k6";

const TARGET = __ENV.TARGET || "http://localhost:8080";
const POSITION_ID = __ENV.POSITION_ID || "";
const COOKIE = __ENV.COOKIE || "";
const BEARER = __ENV.BEARER || "";
const SAMPLE = __ENV.SAMPLE || "./loadtest/sample.pdf";
const CVS = parseInt(__ENV.CVS || "100", 10);
const VUS = parseInt(__ENV.VUS || "10", 10);

const cvBin = open(SAMPLE, "b");
const cvName = SAMPLE.split("/").pop();

export const options = {
  scenarios: {
    intake_batch: {
      executor: "shared-iterations",
      vus: VUS,
      iterations: CVS,
      maxDuration: "10m",
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"], // < 1% intake errors
    http_req_duration: ["p(95)<3000"], // intake (upload+enqueue) p95 < 3s
  },
};

export default function () {
  if (!POSITION_ID) throw new Error("POSITION_ID env is required (a valid position uuid)");
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
}
