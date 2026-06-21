package breach

import (
	"fmt"
	"strings"
	"time"
)

// bangkok is the reporting timezone for human-readable timestamps in the
// generated notification. Falls back to a fixed +07:00 if tzdata is unavailable.
var bangkok = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Bangkok"); err == nil {
		return loc
	}
	return time.FixedZone("ICT", 7*60*60)
}()

// DPOContact is the controller + Data Protection Officer contact block printed in
// the notification. The DPO fields are wired from config in Phase 5.4; until then
// the generator substitutes a visible placeholder so the draft is never silently
// missing the s.41 contact.
type DPOContact struct {
	Company  string
	DPOName  string
	DPOEmail string
	DPOPhone string
}

func orPlaceholder(v, placeholder string) string {
	if strings.TrimSpace(v) == "" {
		return placeholder
	}
	return v
}

func fmtBKK(t time.Time) string {
	return t.In(bangkok).Format("2 Jan 2006 15:04") + " (ICT)"
}

// Notification is the generated PDPC breach-notification draft. It is bilingual
// (Thai is authoritative for the PDPC) and carries the same facts as a
// machine-readable head so a console can render either.
//
// SECURITY: Body embeds operator-entered breach fields (title, description,
// data_categories, remediation) verbatim. It is plain text. Any renderer that
// displays it as HTML MUST escape it first to avoid stored XSS.
type Notification struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// GenerateNotification renders the s.37(4) PDPC notification content for a breach
// as of `now`. It is a pure function (no I/O, injectable clock) so it is fully
// unit-testable. The content covers the PDPA-mandated elements: nature of the
// breach, categories and approximate number of affected subjects, likely
// consequences, the remediation measures taken, and the DPO contact.
func GenerateNotification(b Breach, c DPOContact, now time.Time) Notification {
	company := orPlaceholder(c.Company, "[ชื่อผู้ควบคุมข้อมูล / Controller name]")
	dpoName := orPlaceholder(c.DPOName, "[ชื่อ DPO / DPO name]")
	dpoEmail := orPlaceholder(c.DPOEmail, "[อีเมล DPO / DPO email]")
	dpoPhone := orPlaceholder(c.DPOPhone, "[เบอร์ DPO / DPO phone]")

	dl := computeDeadline(b.DiscoveredAt, b.PDPCNotifiedAt, now)
	occurred := "ไม่ทราบแน่ชัด / unknown"
	if b.OccurredAt != nil {
		occurred = fmtBKK(*b.OccurredAt)
	}

	highRisk := "ไม่ / No"
	if b.HighRisk {
		highRisk = "ใช่ - ต้องแจ้งเจ้าของข้อมูลโดยไม่ชักช้า / Yes - data subjects must be notified without delay"
	}

	deadlineLine := fmt.Sprintf("กำหนดแจ้ง PDPC ภายใน 72 ชม. / PDPC 72h deadline: %s", fmtBKK(dl.DueAt))
	if dl.Notified && b.PDPCNotifiedAt != nil {
		deadlineLine += fmt.Sprintf("\n  แจ้ง PDPC แล้วเมื่อ / PDPC notified at: %s", fmtBKK(*b.PDPCNotifiedAt))
	} else if dl.Overdue {
		deadlineLine += "\n  ** เกินกำหนด 72 ชม. / OVERDUE **"
	} else {
		deadlineLine += fmt.Sprintf("\n  เหลือเวลา / time remaining: %d ชม. / hours", dl.HoursRemaining)
	}

	subject := fmt.Sprintf("แจ้งเหตุการละเมิดข้อมูลส่วนบุคคล / Personal Data Breach Notification - %s", b.Title)

	var sb strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&sb, format+"\n", a...) }

	w("เรียน คณะกรรมการคุ้มครองข้อมูลส่วนบุคคล (PDPC)")
	w("To: Personal Data Protection Committee (PDPC)")
	w("")
	w("ตามมาตรา 37(4) แห่ง พ.ร.บ. คุ้มครองข้อมูลส่วนบุคคล พ.ศ. 2562")
	w("Pursuant to s.37(4) of the Personal Data Protection Act B.E. 2562")
	w("")
	w("1. ผู้ควบคุมข้อมูล / Data controller: %s", company)
	w("")
	w("2. ลักษณะของเหตุการละเมิด / Nature of the breach:")
	w("   %s", b.Title)
	w("   %s", b.Description)
	w("   ระดับความรุนแรง / Severity: %s", b.Severity)
	w("")
	w("3. ประเภทข้อมูลที่เกี่ยวข้อง / Categories of personal data affected:")
	w("   %s", orPlaceholder(b.DataCategories, "[ระบุประเภทข้อมูล / specify categories]"))
	w("")
	w("4. จำนวนเจ้าของข้อมูลที่ได้รับผลกระทบโดยประมาณ / Approximate number of affected data subjects:")
	w("   %d", b.AffectedSubjects)
	w("")
	w("5. ช่วงเวลา / Timeline:")
	w("   เกิดเหตุ / Occurred: %s", occurred)
	w("   ตรวจพบ / Discovered: %s", fmtBKK(b.DiscoveredAt))
	w("   %s", deadlineLine)
	w("")
	w("6. ความเสี่ยงสูงต่อสิทธิและเสรีภาพของเจ้าของข้อมูล / High risk to data subjects' rights:")
	w("   %s", highRisk)
	w("")
	w("7. มาตรการเยียวยาและป้องกัน / Remediation and preventive measures taken:")
	w("   %s", orPlaceholder(b.Remediation, "[ระบุมาตรการ / specify measures]"))
	w("")
	w("8. ผู้ติดต่อ (เจ้าหน้าที่คุ้มครองข้อมูลส่วนบุคคล) / Contact (Data Protection Officer):")
	w("   %s", dpoName)
	w("   อีเมล / Email: %s", dpoEmail)
	w("   โทร / Phone: %s", dpoPhone)

	return Notification{Subject: subject, Body: strings.TrimRight(sb.String(), "\n")}
}
