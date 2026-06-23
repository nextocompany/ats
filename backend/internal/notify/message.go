package notify

import (
	"fmt"

	"github.com/nexto/hr-ats/pkg/emailtmpl"
)

// Candidate-facing builders. Each event has a LINE builder and an `...EmailMessage`
// twin; both source their content from one shared `...Doc()` so the channels never
// drift. The LINE body is doc.PlainText(); the email additionally carries branded
// HTML rendered from the same Doc.

// StatusMessage builds a candidate-facing LINE notification for a status
// transition. It takes primitives (not domain structs) to keep this package
// decoupled from applications/candidates.
//
// Returns a zero Message (empty Recipient) — which callers skip — when:
//   - the status is not candidate-notifiable, or
//   - the candidate has no LINE handle (legacy/demo, or never logged in via LINE).
func StatusMessage(lineUserID, fullName, status, portalBaseURL string) Message {
	if lineUserID == "" {
		return Message{}
	}
	doc, ok := statusDoc(fullName, status, portalBaseURL)
	if !ok {
		return Message{}
	}
	return Message{
		Channel:   ChannelLINE,
		Recipient: lineUserID,
		Subject:   "อัปเดตสถานะใบสมัคร",
		Body:      doc.PlainText(),
	}
}

// StatusEmailMessage builds the email equivalent of StatusMessage for the same
// candidate-notifiable status set, reusing statusDoc so the LINE and email copy
// never drift. Returns a zero Message (empty Recipient) when the address is empty
// or the status is not notifiable.
func StatusEmailMessage(emailAddr, fullName, status, portalBaseURL string) Message {
	if emailAddr == "" {
		return Message{}
	}
	doc, ok := statusDoc(fullName, status, portalBaseURL)
	if !ok {
		return Message{}
	}
	return Message{
		Channel:   ChannelEmail,
		Recipient: emailAddr,
		Subject:   "อัปเดตสถานะใบสมัคร",
		Body:      doc.PlainText(),
		HTML:      emailtmpl.Render(doc),
	}
}

// docTypeLabelTH maps onboarding document types to Thai labels for notification
// copy. Kept local to the notify package (decoupled from applications).
var docTypeLabelTH = map[string]string{
	"id_card":               "บัตรประชาชน",
	"house_registration":    "ทะเบียนบ้าน",
	"education_certificate": "วุฒิการศึกษา",
	"bank_book":             "สมุดบัญชีธนาคาร",
	"tax_document":          "เอกสารภาษี/ลดหย่อน",
	"photo":                 "รูปถ่าย",
	"health_check":          "ใบรับรองแพทย์",
	"military_certificate":  "หลักฐานทางทหาร (สด.43)",
	"name_change":           "ใบเปลี่ยนชื่อ-สกุล",
}

// docTypeLabel returns the Thai label for an onboarding document type, or a
// generic fallback for an unknown type.
func docTypeLabel(docType string) string {
	if l, ok := docTypeLabelTH[docType]; ok {
		return l
	}
	return "เอกสาร"
}

// DocumentReviewedMessage builds a candidate-facing LINE notification for an
// onboarding-document review outcome (3.8). Returns a zero Message (empty
// Recipient, skipped by callers) when the candidate has no LINE handle.
func DocumentReviewedMessage(lineUserID, fullName, docType string, approved bool, reason, portalBaseURL string) Message {
	if lineUserID == "" {
		return Message{}
	}
	return Message{
		Channel:   ChannelLINE,
		Recipient: lineUserID,
		Subject:   "อัปเดตเอกสาร onboarding",
		Body:      documentReviewedDoc(fullName, docType, approved, reason, portalBaseURL).PlainText(),
	}
}

// DocumentReviewedEmailMessage is the email equivalent of DocumentReviewedMessage,
// reusing the same Doc so the LINE and email copy never drift. Returns a zero
// Message when the address is empty.
func DocumentReviewedEmailMessage(emailAddr, fullName, docType string, approved bool, reason, portalBaseURL string) Message {
	if emailAddr == "" {
		return Message{}
	}
	doc := documentReviewedDoc(fullName, docType, approved, reason, portalBaseURL)
	return Message{
		Channel:   ChannelEmail,
		Recipient: emailAddr,
		Subject:   "อัปเดตเอกสาร onboarding",
		Body:      doc.PlainText(),
		HTML:      emailtmpl.Render(doc),
	}
}

// InvalidResumeMessage builds a candidate-facing LINE notification when an
// uploaded file is detected as not being a resume/CV. Gentle + recoverable: the
// detection can be wrong, so the copy says "อาจไม่ใช่" and points to re-upload.
// Returns a zero Message (skipped by callers) when the candidate has no LINE handle.
func InvalidResumeMessage(lineUserID, fullName, portalBaseURL string) Message {
	if lineUserID == "" {
		return Message{}
	}
	return Message{
		Channel:   ChannelLINE,
		Recipient: lineUserID,
		Subject:   "กรุณาอัปโหลดเรซูเม่อีกครั้ง",
		Body:      invalidResumeDoc(fullName, portalBaseURL).PlainText(),
	}
}

// InvalidResumeEmailMessage is the email equivalent of InvalidResumeMessage,
// reusing the same Doc so the LINE and email copy never drift. Returns a zero
// Message when the address is empty.
func InvalidResumeEmailMessage(emailAddr, fullName, portalBaseURL string) Message {
	if emailAddr == "" {
		return Message{}
	}
	doc := invalidResumeDoc(fullName, portalBaseURL)
	return Message{
		Channel:   ChannelEmail,
		Recipient: emailAddr,
		Subject:   "กรุณาอัปโหลดเรซูเม่อีกครั้ง",
		Body:      doc.PlainText(),
		HTML:      emailtmpl.Render(doc),
	}
}

// ApplicationReceivedMessage is sent right after a successful apply: it confirms
// which position the candidate applied to and the current status. position is the
// human-readable title (may be empty). Zero Message when there is no LINE handle.
func ApplicationReceivedMessage(lineUserID, fullName, position, portalBaseURL string) Message {
	if lineUserID == "" {
		return Message{}
	}
	return Message{
		Channel:   ChannelLINE,
		Recipient: lineUserID,
		Subject:   "ได้รับใบสมัครของคุณแล้ว",
		Body:      applicationReceivedDoc(fullName, position, portalBaseURL).PlainText(),
	}
}

// ApplicationReceivedEmailMessage is the email equivalent, sharing the Doc.
func ApplicationReceivedEmailMessage(emailAddr, fullName, position, portalBaseURL string) Message {
	if emailAddr == "" {
		return Message{}
	}
	doc := applicationReceivedDoc(fullName, position, portalBaseURL)
	return Message{
		Channel:   ChannelEmail,
		Recipient: emailAddr,
		Subject:   "ได้รับใบสมัครของคุณแล้ว",
		Body:      doc.PlainText(),
		HTML:      emailtmpl.Render(doc),
	}
}

func applicationReceivedDoc(fullName, position, portalBaseURL string) emailtmpl.Doc {
	pos := position
	if pos == "" {
		pos = "ที่คุณสมัคร"
	}
	return emailtmpl.Doc{
		Title:      "ได้รับใบสมัครของคุณแล้ว",
		Greeting:   emailtmpl.Greeting(fullName),
		Paragraphs: []string{fmt.Sprintf("เราได้รับใบสมัครงานตำแหน่ง \"%s\" ของคุณเรียบร้อยแล้ว", pos)},
		Details:    []emailtmpl.DetailRow{{Label: "สถานะปัจจุบัน", Value: "รอการตรวจสอบ"}},
		CTA:        &emailtmpl.CTA{Label: "ติดตามสถานะใบสมัคร", URL: portalBaseURL + "/status"},
	}
}

// NameMismatchMessage builds a candidate-facing LINE notification when the name
// parsed from the uploaded resume does not match the account holder's name.
// Gentle + recoverable (the match can be imperfect): asks them to re-upload their
// own CV. Zero Message (skipped) when there is no LINE handle.
func NameMismatchMessage(lineUserID, fullName, portalBaseURL string) Message {
	if lineUserID == "" {
		return Message{}
	}
	return Message{
		Channel:   ChannelLINE,
		Recipient: lineUserID,
		Subject:   "กรุณาอัปโหลดเรซูเม่ของคุณเอง",
		Body:      nameMismatchDoc(fullName, portalBaseURL).PlainText(),
	}
}

// NameMismatchEmailMessage is the email equivalent of NameMismatchMessage,
// reusing the same Doc so the two channels never drift.
func NameMismatchEmailMessage(emailAddr, fullName, portalBaseURL string) Message {
	if emailAddr == "" {
		return Message{}
	}
	doc := nameMismatchDoc(fullName, portalBaseURL)
	return Message{
		Channel:   ChannelEmail,
		Recipient: emailAddr,
		Subject:   "กรุณาอัปโหลดเรซูเม่ของคุณเอง",
		Body:      doc.PlainText(),
		HTML:      emailtmpl.Render(doc),
	}
}

func nameMismatchDoc(fullName, portalBaseURL string) emailtmpl.Doc {
	return emailtmpl.Doc{
		Title:    "กรุณาอัปโหลดเรซูเม่ของคุณเอง",
		Greeting: emailtmpl.Greeting(fullName),
		Paragraphs: []string{
			"ชื่อในไฟล์เรซูเม่ที่อัปโหลดดูเหมือนจะไม่ตรงกับชื่อบัญชีของคุณ จึงยังไม่สามารถพิจารณาใบสมัครได้ " +
				"กรุณาตรวจสอบและอัปโหลดเรซูเม่ของคุณเองอีกครั้ง",
		},
		CTA: &emailtmpl.CTA{Label: "อัปโหลดเรซูเม่อีกครั้ง", URL: portalBaseURL + "/status"},
	}
}

func invalidResumeDoc(fullName, portalBaseURL string) emailtmpl.Doc {
	return emailtmpl.Doc{
		Title:    "กรุณาอัปโหลดเรซูเม่อีกครั้ง",
		Greeting: emailtmpl.Greeting(fullName),
		Paragraphs: []string{
			"ไฟล์ที่คุณอัปโหลดอาจไม่ใช่เรซูเม่/CV จึงยังไม่สามารถพิจารณาใบสมัครของคุณได้ " +
				"กรุณาตรวจสอบและอัปโหลดไฟล์เรซูเม่ของคุณอีกครั้ง",
		},
		CTA: &emailtmpl.CTA{Label: "อัปโหลดเรซูเม่อีกครั้ง", URL: portalBaseURL + "/status"},
	}
}

func documentReviewedDoc(fullName, docType string, approved bool, reason, portalBaseURL string) emailtmpl.Doc {
	label := docTypeLabel(docType)
	doc := emailtmpl.Doc{
		Title:    "อัปเดตเอกสาร onboarding",
		Greeting: emailtmpl.Greeting(fullName),
		CTA:      &emailtmpl.CTA{Label: "ดูรายละเอียดเอกสาร", URL: portalBaseURL + "/account"},
	}
	if approved {
		doc.Paragraphs = []string{fmt.Sprintf("เอกสาร \"%s\" ของคุณได้รับการอนุมัติแล้ว", label)}
		return doc
	}
	doc.Paragraphs = []string{fmt.Sprintf("เอกสาร \"%s\" ของคุณถูกตีกลับ กรุณาอัปโหลดใหม่", label)}
	doc.Details = []emailtmpl.DetailRow{{Label: "เหตุผล", Value: fallback(reason, "-")}}
	doc.Accent = emailtmpl.AccentDanger
	return doc
}

// statusDoc maps a candidate-notifiable status to its email/LINE content. The
// second return is false for internal-only states (pending/parsed/failed and any
// unknown value) which are not candidate-facing here (apply sends its own
// "received" message; the invalid_resume / name_mismatch gates send their own).
// The Message.Subject stays "อัปเดตสถานะใบสมัคร" across statuses; Doc.Title is the
// per-status HTML heading (it does not affect the plain body, so no drift).
func statusDoc(fullName, status, portalBaseURL string) (emailtmpl.Doc, bool) {
	base := func(title string, paras ...string) emailtmpl.Doc {
		return emailtmpl.Doc{
			Title:      title,
			Greeting:   emailtmpl.Greeting(fullName),
			Paragraphs: paras,
			CTA:        &emailtmpl.CTA{Label: "ตรวจสอบสถานะใบสมัคร", URL: portalBaseURL + "/status"},
		}
	}
	switch status {
	case "scored":
		return base("ใบสมัครผ่านการคัดกรองเบื้องต้น",
			"ใบสมัครของคุณผ่านการคัดกรองเบื้องต้นเรียบร้อยแล้ว และอยู่ระหว่างการพิจารณาของทีม HR"), true
	case "ai_interview":
		return base("เชิญทำแบบสัมภาษณ์ AI เบื้องต้น",
			"คุณได้รับเชิญให้ทำแบบสัมภาษณ์เบื้องต้นกับผู้ช่วย AI กรุณาทำให้เสร็จเพื่อเข้าสู่ขั้นตอนถัดไป"), true
	case "ai_interviewed":
		return base("ได้รับแบบสัมภาษณ์ของคุณแล้ว",
			"เราได้รับแบบสัมภาษณ์เบื้องต้นของคุณแล้ว ทีมงานกำลังพิจารณาผล"), true
	case "shortlisted":
		return base("เข้าสู่รอบพิจารณาคัดเลือก",
			"ใบสมัครของคุณเข้าสู่รอบพิจารณาคัดเลือก ทีม HR จะติดต่อกลับ"), true
	case "interview":
		return base("คุณได้รับเชิญเข้าสัมภาษณ์",
			"คุณได้รับเชิญเข้าสัมภาษณ์ ทีมงานจะติดต่อเพื่อนัดหมายเร็ว ๆ นี้"), true
	case "interviewed":
		return base("การสัมภาษณ์เสร็จสิ้นแล้ว",
			"การสัมภาษณ์ของคุณเสร็จสิ้นแล้ว ทีมงานกำลังพิจารณาผลและจะแจ้งให้ทราบ"), true
	case "pending_approval":
		return base("อยู่ระหว่างการอนุมัติการจ้าง",
			"ใบสมัครของคุณอยู่ระหว่างขั้นตอนการอนุมัติการจ้างภายใน เราจะแจ้งผลให้ทราบเร็ว ๆ นี้"), true
	case "offer":
		d := base("คุณได้รับข้อเสนอการจ้างงาน",
			"คุณได้รับข้อเสนอการจ้างงาน! เข้าสู่ระบบเพื่อดูรายละเอียดและตอบรับข้อเสนอ")
		d.CTA = &emailtmpl.CTA{Label: "ดูข้อเสนอการจ้างงาน", URL: portalBaseURL + "/offers"}
		return d, true
	case "hired":
		d := base("ยินดีด้วย คุณได้รับการคัดเลือก",
			"ยินดีด้วย! คุณได้รับการคัดเลือก กรุณาอัปโหลดเอกสารเริ่มงานของคุณ ทีม HR จะติดต่อกลับเรื่องวันเริ่มงาน")
		d.CTA = &emailtmpl.CTA{Label: "อัปโหลดเอกสารเริ่มงาน", URL: portalBaseURL + "/account"}
		return d, true
	case "rejected":
		d := base("อัปเดตสถานะใบสมัคร",
			"ขอบคุณที่ให้ความสนใจร่วมงานกับเรา ใบสมัครของคุณยังไม่ผ่านการพิจารณาในรอบนี้ เราจะเก็บข้อมูลไว้พิจารณาในโอกาสต่อไป")
		d.Accent = emailtmpl.AccentDanger
		return d, true
	default:
		return emailtmpl.Doc{}, false
	}
}
