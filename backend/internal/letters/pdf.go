// Package letters renders bilingual (Thai) PDF letters — interview invitations and
// offer letters — for the ATS (Module-3 3.3). Pure-Go via go-pdf/fpdf with an
// embedded Sarabun TTF (SIL OFL) so Thai glyphs render without a system font or
// headless browser. The renderer is pure: LetterData in, PDF bytes out.
package letters

import (
	"bytes"
	_ "embed"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
)

//go:embed fonts/Sarabun-Regular.ttf
var sarabunRegular []byte

//go:embed fonts/Sarabun-Bold.ttf
var sarabunBold []byte

// Letter types.
const (
	TypeInterview = "interview"
	TypeOffer     = "offer"
)

// InterviewDetails carries the interview-letter body fields.
type InterviewDetails struct {
	ScheduledAt time.Time
	DurationMin int
	Mode        string // onsite | online
	Location    string
	JoinURL     string
}

// OfferDetails carries the offer-letter body fields.
type OfferDetails struct {
	Salary    float64
	StartDate time.Time
	Terms     string
}

// LetterData is everything the renderer needs (company name is supplied by the
// Renderer, not the caller).
type LetterData struct {
	Type          string
	CandidateName string
	PositionTitle string
	StoreName     string
	IssuedDate    time.Time
	Interview     *InterviewDetails
	Offer         *OfferDetails
}

// Renderer produces letter PDFs with a fixed company letterhead.
type Renderer struct{ companyName string }

// NewRenderer builds a letter renderer for the given company name.
func NewRenderer(companyName string) *Renderer {
	if companyName == "" {
		companyName = "CP AXTRA"
	}
	return &Renderer{companyName: companyName}
}

var thaiMonths = [...]string{
	"มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน", "พฤษภาคม", "มิถุนายน",
	"กรกฎาคม", "สิงหาคม", "กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม",
}

// thaiDate formats a date in Thai Buddhist-era long form, e.g. "2 มกราคม 2569".
func thaiDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), thaiMonths[int(t.Month())-1], t.Year()+543)
}

// Render builds the PDF for the given letter and returns its bytes.
func (r *Renderer) Render(d LetterData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("Sarabun", "", sarabunRegular)
	pdf.AddUTF8FontFromBytes("Sarabun", "B", sarabunBold)
	pdf.SetMargins(20, 20, 20)
	pdf.SetAutoPageBreak(true, 20)
	pdf.AddPage()

	// Letterhead.
	pdf.SetFont("Sarabun", "B", 18)
	pdf.CellFormat(0, 10, r.companyName, "", 1, "C", false, 0, "")
	pdf.SetFont("Sarabun", "", 11)
	pdf.CellFormat(0, 6, "ฝ่ายทรัพยากรบุคคล (Human Resources)", "", 1, "C", false, 0, "")
	pdf.Ln(6)

	// Date (right-aligned, Thai B.E.).
	issued := d.IssuedDate
	if issued.IsZero() {
		issued = time.Now()
	}
	pdf.SetFont("Sarabun", "", 12)
	pdf.CellFormat(0, 7, "วันที่ "+thaiDate(issued), "", 1, "R", false, 0, "")
	pdf.Ln(2)

	// Salutation.
	name := d.CandidateName
	if name == "" {
		name = "ผู้สมัคร"
	}
	pdf.SetFont("Sarabun", "B", 12)
	pdf.CellFormat(0, 7, "เรียน คุณ"+name, "", 1, "L", false, 0, "")
	pdf.Ln(1)

	pdf.SetFont("Sarabun", "", 12)
	switch d.Type {
	case TypeInterview:
		if d.Interview == nil {
			return nil, fmt.Errorf("letters: interview letter requires interview details")
		}
		r.interviewBody(pdf, d)
	case TypeOffer:
		if d.Offer == nil {
			return nil, fmt.Errorf("letters: offer letter requires offer details")
		}
		r.offerBody(pdf, d)
	default:
		return nil, fmt.Errorf("letters: unknown letter type %q", d.Type)
	}

	// Signature block.
	pdf.Ln(10)
	pdf.CellFormat(0, 7, "ขอแสดงความนับถือ", "", 1, "L", false, 0, "")
	pdf.Ln(12)
	pdf.SetFont("Sarabun", "B", 12)
	pdf.CellFormat(0, 7, r.companyName, "", 1, "L", false, 0, "")
	pdf.SetFont("Sarabun", "", 11)
	pdf.CellFormat(0, 6, "ฝ่ายทรัพยากรบุคคล", "", 1, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("letters: render pdf: %w", err)
	}
	return buf.Bytes(), nil
}

func (r *Renderer) interviewBody(pdf *fpdf.Fpdf, d LetterData) {
	pos := fallback(d.PositionTitle, "ที่ท่านสมัคร")
	pdf.MultiCell(0, 7, fmt.Sprintf(
		"ตามที่ท่านได้สมัครงานในตำแหน่ง %s นั้น บริษัทฯ ขอเรียนเชิญท่านเข้ารับการสัมภาษณ์งาน โดยมีรายละเอียดดังนี้",
		pos), "", "L", false)
	pdf.Ln(2)
	if d.Interview != nil {
		iv := d.Interview
		line(pdf, "วันและเวลา", fmt.Sprintf("%s เวลา %02d:%02d น.", thaiDate(iv.ScheduledAt), iv.ScheduledAt.Hour(), iv.ScheduledAt.Minute()))
		if iv.DurationMin > 0 {
			line(pdf, "ระยะเวลา", fmt.Sprintf("ประมาณ %d นาที", iv.DurationMin))
		}
		if iv.Mode == "online" {
			line(pdf, "รูปแบบ", "สัมภาษณ์ออนไลน์")
			if iv.JoinURL != "" {
				line(pdf, "ลิงก์เข้าร่วม", iv.JoinURL)
			}
		} else {
			line(pdf, "รูปแบบ", "สัมภาษณ์ ณ สถานที่")
			if iv.Location != "" {
				line(pdf, "สถานที่", iv.Location)
			}
		}
	}
	if d.StoreName != "" {
		line(pdf, "หน่วยงาน", d.StoreName)
	}
	pdf.Ln(2)
	pdf.MultiCell(0, 7, "กรุณายืนยันการเข้าสัมภาษณ์กับเจ้าหน้าที่ฝ่ายบุคคล บริษัทฯ หวังเป็นอย่างยิ่งว่าจะได้พบท่านตามวันและเวลาดังกล่าว", "", "L", false)
}

func (r *Renderer) offerBody(pdf *fpdf.Fpdf, d LetterData) {
	pos := fallback(d.PositionTitle, "ที่ท่านสมัคร")
	store := d.StoreName
	intro := fmt.Sprintf("บริษัทฯ มีความยินดีขอเสนอการจ้างงานแก่ท่าน ในตำแหน่ง %s", pos)
	if store != "" {
		intro += fmt.Sprintf(" ประจำ %s", store)
	}
	pdf.MultiCell(0, 7, intro+" โดยมีรายละเอียดของข้อเสนอดังนี้", "", "L", false)
	pdf.Ln(2)
	if d.Offer != nil {
		of := d.Offer
		line(pdf, "ตำแหน่ง", pos)
		if of.Salary > 0 {
			line(pdf, "อัตราเงินเดือน", fmt.Sprintf("%s บาท/เดือน", humanizeTHB(of.Salary)))
		}
		if !of.StartDate.IsZero() {
			line(pdf, "วันเริ่มงาน", thaiDate(of.StartDate))
		}
		if of.Terms != "" {
			pdf.Ln(1)
			pdf.SetFont("Sarabun", "B", 12)
			pdf.CellFormat(0, 7, "เงื่อนไขเพิ่มเติม", "", 1, "L", false, 0, "")
			pdf.SetFont("Sarabun", "", 12)
			pdf.MultiCell(0, 7, of.Terms, "", "L", false)
		}
	}
	pdf.Ln(2)
	pdf.MultiCell(0, 7, "หากท่านประสงค์จะตอบรับข้อเสนอนี้ กรุณายืนยันผ่านระบบรับสมัครงานออนไลน์ หรือติดต่อฝ่ายบุคคล บริษัทฯ ยินดีต้อนรับท่านเข้าร่วมเป็นส่วนหนึ่งขององค์กร", "", "L", false)
}

// line writes a "label: value" row with a bold label.
func line(pdf *fpdf.Fpdf, label, value string) {
	pdf.SetFont("Sarabun", "B", 12)
	pdf.CellFormat(45, 7, label, "", 0, "L", false, 0, "")
	pdf.SetFont("Sarabun", "", 12)
	pdf.MultiCell(0, 7, value, "", "L", false)
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// humanizeTHB formats a baht amount with thousands separators, no decimals.
func humanizeTHB(v float64) string {
	n := int64(v)
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
