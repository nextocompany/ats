// Zero-dependency mock of the public Career API — just enough for the GAN design
// harness to render /jobs and /jobs/[id] with realistic CP Axtra retail content.
// Envelope shape + CORS-with-credentials mirror the real Go backend so the portal
// behaves exactly as in prod (minus auth/apply, which the design eval never submits).
import { createServer } from "node:http";

const PORT = process.env.MOCK_PORT ? Number(process.env.MOCK_PORT) : 8090;

// Realistic open positions across a Thai retail group (Makro / Lotus's flavour).
const POSITIONS = [
  { id: "p01", title_th: "พนักงานประจำสาขา (พาร์ทไทม์/เต็มเวลา)", title_en: "Store Associate", level: "entry", open_count: 48 },
  { id: "p02", title_th: "แคชเชียร์", title_en: "Cashier", level: "entry", open_count: 32 },
  { id: "p03", title_th: "พนักงานจัดเรียงสินค้า", title_en: "Merchandiser", level: "entry", open_count: 27 },
  { id: "p04", title_th: "ผู้ช่วยผู้จัดการแผนกอาหารสด", title_en: "Assistant Fresh Food Manager", level: "experienced", open_count: 14 },
  { id: "p05", title_th: "หัวหน้าแผนกคลังสินค้า", title_en: "Warehouse Supervisor", level: "experienced", open_count: 9 },
  { id: "p06", title_th: "ผู้จัดการสาขา", title_en: "Store Manager", level: "management", open_count: 6 },
  { id: "p07", title_th: "เจ้าหน้าที่ความปลอดภัยอาชีวอนามัย", title_en: "Safety Officer", level: "experienced", open_count: 5 },
  { id: "p08", title_th: "พนักงานขับรถส่งสินค้า", title_en: "Delivery Driver", level: "entry", open_count: 21 },
  { id: "p09", title_th: "นักวิเคราะห์ข้อมูลค้าปลีก", title_en: "Retail Data Analyst", level: "experienced", open_count: 4 },
  { id: "p10", title_th: "วิศวกรซอฟต์แวร์ (อีคอมเมิร์ซ)", title_en: "Software Engineer, E-commerce", level: "senior", open_count: 7 },
  { id: "p11", title_th: "ผู้จัดการฝ่ายทรัพยากรบุคคล", title_en: "HR Manager", level: "management", open_count: 3 },
  { id: "p12", title_th: "เจ้าหน้าที่การตลาดดิจิทัล", title_en: "Digital Marketing Officer", level: "experienced", open_count: 8 },
];

function send(res, origin, status, body) {
  res.writeHead(status, {
    "Content-Type": "application/json; charset=utf-8",
    "Access-Control-Allow-Origin": origin || "*",
    "Access-Control-Allow-Credentials": "true",
    "Access-Control-Allow-Methods": "GET,POST,PATCH,OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type",
  });
  res.end(JSON.stringify(body));
}

const server = createServer((req, res) => {
  const origin = req.headers.origin || "*";
  const url = new URL(req.url, `http://localhost:${PORT}`);
  const path = url.pathname;

  if (req.method === "OPTIONS") return send(res, origin, 204, {});

  // List
  if (path === "/api/v1/public/positions") {
    return send(res, origin, 200, {
      success: true,
      data: POSITIONS,
      meta: { total: POSITIONS.length, page: 1, limit: 100 },
    });
  }

  // Detail
  const m = path.match(/^\/api\/v1\/public\/positions\/([^/]+)$/);
  if (m) {
    const p = POSITIONS.find((x) => x.id === m[1]);
    if (!p) return send(res, origin, 404, { success: false, data: null, error: "not found" });
    const { id, title_th, title_en, level } = p;
    return send(res, origin, 200, { success: true, data: { id, title_th, title_en, level } });
  }

  // Logged-out account probe — the portal treats 401 as "not signed in".
  if (path === "/api/v1/public/auth/me") {
    return send(res, origin, 401, { success: false, data: null, error: "unauthenticated" });
  }

  // Health
  if (path === "/health") return send(res, origin, 200, { success: true, data: "ok" });

  return send(res, origin, 404, { success: false, data: null, error: "not found" });
});

server.listen(PORT, () => {
  // eslint-disable-next-line no-console
  console.log(`[mock-api] listening on http://localhost:${PORT} — ${POSITIONS.length} positions`);
});
