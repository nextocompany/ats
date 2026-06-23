package emailtmpl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGreeting(t *testing.T) {
	if g := Greeting(""); g != "สวัสดีค่ะ" {
		t.Errorf("empty name: got %q", g)
	}
	if g := Greeting("สมชาย"); g != "สวัสดีคุณสมชาย" {
		t.Errorf("named: got %q", g)
	}
}

func TestPlainText_Stable(t *testing.T) {
	d := Doc{
		Title:      "ได้รับใบสมัครของคุณแล้ว",
		Greeting:   "สวัสดีคุณสมชาย",
		Paragraphs: []string{"เราได้รับใบสมัครของคุณเรียบร้อยแล้ว"},
		Details:    []DetailRow{{Label: "สถานะปัจจุบัน", Value: "รอการตรวจสอบ"}},
		CTA:        &CTA{Label: "ติดตามสถานะ", URL: "https://careers.example.com/status"},
	}
	want := "สวัสดีคุณสมชาย\n\n" +
		"เราได้รับใบสมัครของคุณเรียบร้อยแล้ว\n\n" +
		"สถานะปัจจุบัน: รอการตรวจสอบ\n\n" +
		"ติดตามสถานะ: https://careers.example.com/status"
	if got := d.PlainText(); got != want {
		t.Errorf("PlainText drift:\n got: %q\nwant: %q", got, want)
	}
}

func TestRender_Structure(t *testing.T) {
	d := Doc{
		Title:      "นัดหมายสัมภาษณ์",
		Greeting:   "สวัสดีคุณสมชาย",
		Paragraphs: []string{"นัดสัมภาษณ์ของคุณ"},
		Details:    []DetailRow{{Label: "📅 วันเวลา", Value: "25 มิถุนายน 2569 เวลา 14:00 น."}},
		CTA:        &CTA{Label: "ดูรายละเอียด", URL: "https://careers.example.com/status"},
	}
	out := Render(d)
	for _, want := range []string{
		"CP", "AXTRA", "Careers", // wordmark
		"นัดหมายสัมภาษณ์",                    // title
		"สวัสดีคุณสมชาย",                     // greeting
		"นัดสัมภาษณ์ของคุณ",                  // paragraph
		"📅 วันเวลา",                          // detail label w/ emoji
		"25 มิถุนายน 2569",                   // detail value
		"ดูรายละเอียด",                       // CTA label
		"https://careers.example.com/status", // CTA href
		"cpaxtra.com",                        // footer
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Render missing %q", want)
		}
	}
}

func TestRender_Escaping(t *testing.T) {
	d := Doc{
		Title:      "ทดสอบ",
		Greeting:   "สวัสดีคุณ<script>alert(1)</script>",
		Paragraphs: []string{"เหตุผล: <b>x</b> & ปลอดภัย"},
	}
	out := Render(d)
	if strings.Contains(out, "<script>") {
		t.Error("raw <script> leaked into HTML (XSS)")
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Error("script tag not escaped")
	}
	if !strings.Contains(out, "&lt;b&gt;") {
		t.Error("bold tag not escaped")
	}
}

func TestRender_CTASchemeRejected(t *testing.T) {
	d := Doc{Title: "x", CTA: &CTA{Label: "คลิก", URL: "javascript:alert(1)"}}
	out := Render(d)
	if strings.Contains(out, "javascript:") {
		t.Error("javascript: URL must not appear in rendered HTML")
	}
	if strings.Contains(out, "คลิก →") {
		t.Error("button should be dropped for a non-http CTA")
	}
}

func TestRender_NoZgotmplZ(t *testing.T) {
	cases := []struct {
		accent  string
		wantHex string
	}{
		{AccentDefault, "#0B47B8"},
		{AccentDanger, "#D64545"},
		{AccentWarning, "#D99A2B"},
	}
	for _, c := range cases {
		out := Render(Doc{Title: "x", Accent: c.accent})
		if strings.Contains(out, "ZgotmplZ") {
			t.Errorf("accent %q: html/template neutered the style (ZgotmplZ present)", c.accent)
		}
		if !strings.Contains(out, c.wantHex) {
			t.Errorf("accent %q: missing rule color %s", c.accent, c.wantHex)
		}
	}
}

func TestRenderPlain_Fallback(t *testing.T) {
	out := RenderPlain("สวัสดีคุณสมชาย\nดูได้ที่ https://careers.example.com/status")
	if !strings.Contains(out, "CP") || !strings.Contains(out, "AXTRA") {
		t.Error("plain fallback missing wordmark shell")
	}
	if !strings.Contains(out, `href="https://careers.example.com/status"`) {
		t.Error("plain fallback did not linkify the URL")
	}
	// A bare body with metacharacters must be escaped.
	if got := RenderPlain("<script>x</script>"); strings.Contains(got, "<script>x") {
		t.Error("plain fallback leaked raw HTML")
	}
}

// TestWritePreviews renders representative Docs to testdata/preview/*.html for
// manual visual inspection. Skipped in -short; harmless otherwise.
func TestWritePreviews(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping preview write in -short")
	}
	dir := filepath.Join("testdata", "preview")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	previews := map[string]Doc{
		"application-received": {
			Title:      "ได้รับใบสมัครของคุณแล้ว",
			Greeting:   Greeting("สมชาย ใจดี"),
			Paragraphs: []string{`เราได้รับใบสมัครงานตำแหน่ง "แคชเชียร์" ของคุณเรียบร้อยแล้ว`},
			Details:    []DetailRow{{Label: "สถานะปัจจุบัน", Value: "รอการตรวจสอบ"}},
			CTA:        &CTA{Label: "ติดตามสถานะใบสมัคร", URL: "https://careers.example.com/status"},
		},
		"status-hired": {
			Title:      "ยินดีด้วย คุณได้รับการคัดเลือก",
			Greeting:   Greeting("สมหญิง"),
			Paragraphs: []string{"กรุณาอัปโหลดเอกสารเริ่มงานของคุณ ทีม HR จะติดต่อกลับเรื่องวันเริ่มงาน"},
			CTA:        &CTA{Label: "อัปโหลดเอกสารเริ่มงาน", URL: "https://careers.example.com/account"},
		},
		"interview-scheduled": {
			Title:    "นัดหมายสัมภาษณ์",
			Greeting: Greeting("สมชาย"),
			Details: []DetailRow{
				{Label: "📅 วันเวลา", Value: "25 มิถุนายน 2569 เวลา 14:00 น. (ประมาณ 30 นาที)"},
				{Label: "📍 รูปแบบ", Value: "สัมภาษณ์ที่สถานที่"},
				{Label: "สถานที่", Value: "CP Axtra สาขาเชียงใหม่ ชั้น 3"},
			},
			CTA: &CTA{Label: "ดูรายละเอียด", URL: "https://careers.example.com/status"},
		},
		"hr-scored": {
			Title:      "ผู้สมัครใหม่ผ่านการคัดกรอง",
			Paragraphs: []string{"มีผู้สมัครใหม่ผ่านการคัดกรองและถูกจัดให้สาขาของคุณ"},
			Details: []DetailRow{
				{Label: "ผู้สมัคร", Value: "สมชาย ใจดี"},
				{Label: "ตำแหน่ง", Value: "แคชเชียร์"},
				{Label: "คะแนน AI", Value: "82/100"},
			},
			CTA: &CTA{Label: "ดูรายละเอียด", URL: "https://hr.example.com/applications/123"},
		},
		"otp": {
			Title:      "รหัสยืนยันการเข้าสู่ระบบ / Your login code",
			Paragraphs: []string{"ใช้รหัสนี้เพื่อเข้าสู่ระบบ / Use this code to sign in"},
			Details:    []DetailRow{{Label: "รหัสยืนยัน / Code", Value: "482913"}},
			Outro:      "รหัสหมดอายุใน 10 นาที / expires in 10 minutes",
		},
		"rejected": {
			Title:      "อัปเดตสถานะใบสมัคร",
			Greeting:   Greeting("สมชาย"),
			Paragraphs: []string{"ขอบคุณที่ให้ความสนใจร่วมงานกับเรา ใบสมัครของคุณยังไม่ผ่านการพิจารณาในรอบนี้"},
			Accent:     AccentDanger,
			CTA:        &CTA{Label: "ดูตำแหน่งงานอื่น", URL: "https://careers.example.com/jobs"},
		},
	}
	for name, d := range previews {
		if err := os.WriteFile(filepath.Join(dir, name+".html"), []byte(Render(d)), 0o644); err != nil {
			t.Errorf("write %s: %v", name, err)
		}
	}
}
