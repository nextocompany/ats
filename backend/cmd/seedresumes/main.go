// Command seedresumes backfills a sample resume document for every application
// that has none, so the dashboard resume viewer is populated for demos. It
// renders a styled HTML resume (A4-like) from the candidate + position data and
// uploads it through the same blob client the intake path uses, then records the
// stored URL on the application. Idempotent: it only touches applications whose
// raw_file_blob_url is empty.
//
//	DB_URL=... AZURE_BLOB_CONNECTION_STRING=... AZURE_BLOB_CONTAINER=resumes \
//	  go run ./cmd/seedresumes
package main

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"html/template"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/pkg/blob"
)

type row struct {
	ID        string
	Name      string
	Phone     string
	Email     string
	Province  string
	Position  string
	Summary   string
	Status    string
	Score     *float64
}

func main() {
	ctx := context.Background()

	dbURL := mustEnv("DB_URL")
	connStr := mustEnv("AZURE_BLOB_CONNECTION_STRING")
	container := getenv("AZURE_BLOB_CONTAINER", "resumes")

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	bc, err := blob.Connect(ctx, connStr, container)
	if err != nil {
		log.Fatalf("blob connect: %v", err)
	}

	const q = `
		SELECT a.id, c.full_name, COALESCE(c.phone,''), COALESCE(c.email,''),
		       COALESCE(c.province,''), COALESCE(p.title_th, p.title_en, ''),
		       COALESCE(a.ai_summary,''), a.status, a.ai_score
		FROM applications a
		JOIN candidates c ON c.id = a.candidate_id
		JOIN positions  p ON p.id = a.position_id
		WHERE COALESCE(a.raw_file_blob_url,'') = ''`
	rows, err := pool.Query(ctx, q)
	if err != nil {
		log.Fatalf("query: %v", err)
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.Name, &r.Phone, &r.Email, &r.Province,
			&r.Position, &r.Summary, &r.Status, &r.Score); err != nil {
			log.Fatalf("scan: %v", err)
		}
		list = append(list, r)
	}
	rows.Close()

	if len(list) == 0 {
		log.Println("no applications missing a resume — nothing to do")
		return
	}

	done := 0
	for _, r := range list {
		htmlBytes, err := renderResume(r)
		if err != nil {
			log.Printf("skip %s: render: %v", r.ID, err)
			continue
		}
		// Flat blob name (no slash). A "/" in the blob name is percent-encoded in
		// the path, which azurite's SAS validation rejects (signature computed
		// over the un-encoded name → 403). Flat names keep the demo viewer working.
		name := fmt.Sprintf("resume-%s.html", r.ID)
		url, err := bc.Upload(ctx, name, htmlBytes, "text/html; charset=utf-8")
		if err != nil {
			log.Printf("skip %s: upload: %v", r.ID, err)
			continue
		}
		const upd = `UPDATE applications
			SET raw_file_blob_url = $2, raw_file_type = 'html', updated_at = NOW()
			WHERE id = $1`
		if _, err := pool.Exec(ctx, upd, r.ID, url); err != nil {
			log.Printf("skip %s: update: %v", r.ID, err)
			continue
		}
		done++
		log.Printf("seeded resume: %s (%s — %s)", r.Name, r.Position, r.ID)
	}
	log.Printf("done: %d/%d resumes seeded", done, len(list))
}

// --- resume synthesis -------------------------------------------------------

// Small deterministic pools so each candidate gets a believable, stable resume
// derived from their id (no randomness — re-runs are reproducible).
var companies = []string{
	"บมจ. สยามแม็คโคร", "บจก. ซีพี ออลล์", "บมจ. เซ็นทรัล รีเทล",
	"บจก. โลตัส", "บจก. บิ๊กซี ซูเปอร์เซ็นเตอร์", "บจก. โฮมโปร",
}
var schools = []string{
	"มหาวิทยาลัยเชียงใหม่", "มหาวิทยาลัยขอนแก่น", "มหาวิทยาลัยบูรพา",
	"มหาวิทยาลัยรามคำแหง", "มหาวิทยาลัยเทคโนโลยีราชมงคล",
}
var skillPool = []string{
	"ระบบ POS", "บริการลูกค้า", "บริหารสต็อก", "ทำงานเป็นทีม",
	"จัดเรียงสินค้า", "ปิดยอดขายรายวัน", "ดูแลความปลอดภัย", "Microsoft Excel",
}

func pick(seed uint32, pool []string, offset int) string {
	return pool[(int(seed)+offset)%len(pool)]
}

func renderResume(r row) ([]byte, error) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(r.ID))
	seed := h.Sum32()

	years := 2 + int(seed%5) // 2–6 years
	summary := r.Summary
	if strings.TrimSpace(summary) == "" {
		summary = fmt.Sprintf("ผู้สมัครตำแหน่ง%s มีความตั้งใจและพร้อมเรียนรู้งาน", r.Position)
	}

	data := struct {
		row
		Years      int
		CompanyNow string
		CompanyOld string
		School     string
		Skills     []string
		Summary    string
	}{
		row:        r,
		Years:      years,
		CompanyNow: pick(seed, companies, 0),
		CompanyOld: pick(seed, companies, 3),
		School:     pick(seed, schools, 0),
		Skills:     []string{pick(seed, skillPool, 0), pick(seed, skillPool, 2), pick(seed, skillPool, 4), pick(seed, skillPool, 6)},
		Summary:    summary,
	}

	var buf bytes.Buffer
	if err := resumeTmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var resumeTmpl = template.Must(template.New("resume").Parse(`<!doctype html>
<html lang="th"><head><meta charset="utf-8">
<title>เรซูเม่ — {{.Name}}</title>
<style>
  :root{--ink:#16202e;--soft:#5b6678;--brand:#0B47B8;--accent:#FFC02E;--line:#e6eaf2}
  *{box-sizing:border-box;margin:0;padding:0}
  body{background:#eef1f6;font-family:"Sukhumvit Set","IBM Plex Sans Thai","Segoe UI",system-ui,sans-serif;color:var(--ink);padding:28px}
  .page{max-width:780px;margin:0 auto;background:#fff;border-radius:10px;box-shadow:0 14px 40px -18px rgba(11,71,184,.35);overflow:hidden}
  header{background:linear-gradient(135deg,var(--brand),#06307f);color:#fff;padding:28px 36px;position:relative}
  header::after{content:"";position:absolute;right:24px;top:22px;width:64px;height:34px;background-image:radial-gradient(var(--accent) 2px,transparent 2.2px);background-size:12px 12px;opacity:.7}
  header h1{font-size:26px;font-weight:800;letter-spacing:-.01em}
  header .role{margin-top:4px;font-size:14px;opacity:.9}
  header .contact{margin-top:12px;font-size:12.5px;opacity:.92;display:flex;gap:18px;flex-wrap:wrap}
  main{padding:26px 36px 34px}
  section{margin-top:22px}
  section:first-child{margin-top:0}
  h2{font-size:12px;font-weight:800;letter-spacing:.12em;text-transform:uppercase;color:var(--brand);border-bottom:2px solid var(--line);padding-bottom:6px;margin-bottom:12px}
  .lead{font-size:14px;line-height:1.6;color:var(--soft)}
  .job{margin-bottom:14px}
  .job .t{font-weight:700;font-size:14px}
  .job .meta{font-size:12.5px;color:var(--soft);margin-top:1px}
  .job .d{font-size:13px;color:var(--soft);margin-top:4px;line-height:1.5}
  .skills{display:flex;flex-wrap:wrap;gap:8px}
  .chip{font-size:12.5px;background:#eef3fe;color:var(--brand);padding:4px 11px;border-radius:999px;font-weight:600}
  footer{padding:14px 36px;border-top:1px solid var(--line);font-size:11px;color:#98a1b3;font-style:italic}
</style></head>
<body>
  <div class="page">
    <header>
      <h1>{{.Name}}</h1>
      <div class="role">ตำแหน่งที่สนใจ: {{.Position}}</div>
      <div class="contact">
        {{if .Phone}}<span>📞 {{.Phone}}</span>{{end}}
        {{if .Email}}<span>✉ {{.Email}}</span>{{end}}
        {{if .Province}}<span>📍 {{.Province}}</span>{{end}}
      </div>
    </header>
    <main>
      <section>
        <h2>สรุปโดยย่อ</h2>
        <p class="lead">{{.Summary}}</p>
      </section>
      <section>
        <h2>ประสบการณ์ทำงาน</h2>
        <div class="job">
          <div class="t">{{.Position}}</div>
          <div class="meta">{{.CompanyNow}} · {{.Years}} ปี</div>
          <div class="d">รับผิดชอบงานหน้าร้านและบริการลูกค้า ทำงานเป็นกะ ดูแลความเรียบร้อยของพื้นที่ขายและยอดขายประจำวัน</div>
        </div>
        <div class="job">
          <div class="t">พนักงานประจำสาขา</div>
          <div class="meta">{{.CompanyOld}} · {{.Years}} ปีก่อนหน้า</div>
          <div class="d">สนับสนุนงานขายหน้าร้าน จัดเรียงสินค้า และดูแลสต็อกตามมาตรฐานสาขา</div>
        </div>
      </section>
      <section>
        <h2>การศึกษา</h2>
        <div class="job">
          <div class="t">ปริญญาตรี / ปวส.</div>
          <div class="meta">{{.School}}</div>
        </div>
      </section>
      <section>
        <h2>ทักษะ</h2>
        <div class="skills">
          {{range .Skills}}<span class="chip">{{.}}</span>{{end}}
        </div>
      </section>
    </main>
    <footer>เอกสารตัวอย่างสำหรับการสาธิตระบบ — AI HR Recruitment · CP Axtra</footer>
  </div>
</body></html>`))

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("missing required env: %s", k)
	}
	return v
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
