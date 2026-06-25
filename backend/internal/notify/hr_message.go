package notify

import (
	"fmt"

	"github.com/nexto/hr-ats/pkg/emailtmpl"
)

// HR-facing notification builders. Each returns the set of Messages to dispatch:
// one ChannelEmail per recipient address (carrying branded HTML) plus, when
// teamsEnabled, one ChannelTeams message (plain body only — Teams renders its own
// card). Primitives only, to keep this package decoupled from applications/candidates.

// NewScoredHR notifies store HR that a new candidate has been screened + scored and
// assigned to their store. dashURL is the dashboard deep link to the application.
func NewScoredHR(toEmails []string, teamsEnabled bool, candName, positionTitle string, score int, dashURL string) []Message {
	doc := emailtmpl.Doc{
		Title:      "ผู้สมัครใหม่ผ่านการคัดกรอง",
		Paragraphs: []string{"มีผู้สมัครใหม่ผ่านการคัดกรองและถูกจัดให้สาขาของคุณ"},
		Details: []emailtmpl.DetailRow{
			{Label: "ผู้สมัคร", Value: fallback(candName, "ผู้สมัคร")},
			{Label: "ตำแหน่ง", Value: fallback(positionTitle, "-")},
			{Label: "คะแนน AI", Value: fmt.Sprintf("%d/100", score)},
		},
		CTA: &emailtmpl.CTA{Label: "ดูรายละเอียด", URL: dashURL},
	}
	return hrMessages(toEmails, teamsEnabled, doc)
}

// NewCandidateHM notifies a hiring manager that a new candidate has entered a
// requisition they own (a position they opened). The hiring manager is read-only
// on operations but is the hire approver, so this is their cue to review. dashURL
// deep-links the application.
func NewCandidateHM(toEmails []string, teamsEnabled bool, candName, positionTitle string, score int, dashURL string) []Message {
	doc := emailtmpl.Doc{
		Title:      "ผู้สมัครใหม่ในตำแหน่งที่คุณเปิด",
		Paragraphs: []string{"มีผู้สมัครใหม่เข้ามาในตำแหน่งที่คุณเป็นผู้เปิดรับ รอการพิจารณาของคุณ"},
		Details: []emailtmpl.DetailRow{
			{Label: "ผู้สมัคร", Value: fallback(candName, "ผู้สมัคร")},
			{Label: "ตำแหน่ง", Value: fallback(positionTitle, "-")},
			{Label: "คะแนน AI", Value: fmt.Sprintf("%d/100", score)},
		},
		CTA: &emailtmpl.CTA{Label: "ดูรายละเอียด", URL: dashURL},
	}
	return hrMessages(toEmails, teamsEnabled, doc)
}

// AIInterviewPassedHR notifies store HR that a candidate completed the AI
// pre-interview with an actionable score (>= threshold). recommendation is the
// evaluator's verdict (e.g. "strong_recommend"); dashURL deep-links the application.
func AIInterviewPassedHR(toEmails []string, teamsEnabled bool, candName, positionTitle, recommendation string, score int, dashURL string) []Message {
	doc := emailtmpl.Doc{
		Title:      "ผู้สมัครผ่านการสัมภาษณ์ AI เบื้องต้น",
		Paragraphs: []string{"มีผู้สมัครทำการสัมภาษณ์ AI เบื้องต้นเสร็จและได้คะแนนถึงเกณฑ์ที่ควรพิจารณา"},
		Details: []emailtmpl.DetailRow{
			{Label: "ผู้สมัคร", Value: fallback(candName, "ผู้สมัคร")},
			{Label: "ตำแหน่ง", Value: fallback(positionTitle, "-")},
			{Label: "คะแนนสัมภาษณ์ AI", Value: fmt.Sprintf("%d/100", score)},
			{Label: "คำแนะนำ", Value: recommendationLabel(recommendation)},
		},
		CTA: &emailtmpl.CTA{Label: "ดูรายละเอียด", URL: dashURL},
	}
	return hrMessages(toEmails, teamsEnabled, doc)
}

// recommendationLabel renders the AI interview recommendation enum in Thai. An
// unknown value falls back to "-" so a prompt drift never leaks a raw token.
func recommendationLabel(rec string) string {
	switch rec {
	case "strong_recommend":
		return "แนะนำอย่างยิ่ง"
	case "recommend":
		return "แนะนำ"
	case "neutral":
		return "ปานกลาง"
	case "caution":
		return "ควรพิจารณาอย่างระมัดระวัง"
	default:
		return "-"
	}
}

// FeedbackRecordedHR notifies store HR that an interviewer recorded interview
// feedback for a candidate.
func FeedbackRecordedHR(toEmails []string, teamsEnabled bool, candName, positionTitle, interviewer, recommendation, dashURL string) []Message {
	doc := emailtmpl.Doc{
		Title:      "บันทึกผลสัมภาษณ์ใหม่",
		Paragraphs: []string{"มีการบันทึกผลสัมภาษณ์ใหม่"},
		Details: []emailtmpl.DetailRow{
			{Label: "ผู้สมัคร", Value: fallback(candName, "ผู้สมัคร")},
			{Label: "ตำแหน่ง", Value: fallback(positionTitle, "-")},
			{Label: "ผู้สัมภาษณ์", Value: fallback(interviewer, "-")},
			{Label: "ผลสรุป", Value: fallback(recommendation, "-")},
		},
		CTA: &emailtmpl.CTA{Label: "ดูรายละเอียด", URL: dashURL},
	}
	return hrMessages(toEmails, teamsEnabled, doc)
}

// ShortlistReadyLM notifies a store's Line Manager(s) that a candidate has been
// shortlisted and is awaiting their Top-5 review. dashURL deep-links the shortlist.
func ShortlistReadyLM(lmEmails []string, teamsEnabled bool, candName, positionTitle, dashURL string) []Message {
	doc := emailtmpl.Doc{
		Title:      "มีผู้สมัครรอการพิจารณา (Shortlist)",
		Paragraphs: []string{"มีผู้สมัครถูกคัดเข้า shortlist รอการพิจารณาของผู้จัดการสาขา"},
		Details: []emailtmpl.DetailRow{
			{Label: "ผู้สมัคร", Value: fallback(candName, "ผู้สมัคร")},
			{Label: "ตำแหน่ง", Value: fallback(positionTitle, "-")},
		},
		CTA: &emailtmpl.CTA{Label: "ดูรายชื่อคัดสรร", URL: dashURL},
	}
	return hrMessages(lmEmails, teamsEnabled, doc)
}

// ApprovalPendingHR notifies the approvers at a newly active chain level that a
// hire approval is awaiting their decision. levelLabel is e.g. "HR Manager".
func ApprovalPendingHR(toEmails []string, teamsEnabled bool, candName, levelLabel, dashURL string) []Message {
	doc := emailtmpl.Doc{
		Title:      "มีคำขออนุมัติจ้างรอการพิจารณา",
		Paragraphs: []string{"มีคำขออนุมัติการจ้างรอการอนุมัติของคุณ"},
		Details: []emailtmpl.DetailRow{
			{Label: "ผู้สมัคร", Value: fallback(candName, "ผู้สมัคร")},
			{Label: "ขั้นอนุมัติ", Value: fallback(levelLabel, "-")},
		},
		CTA: &emailtmpl.CTA{Label: "ดูรายการอนุมัติ", URL: dashURL},
	}
	return hrMessages(toEmails, teamsEnabled, doc)
}

// ApprovalDecidedHR notifies HR of the final outcome of an approval chain.
func ApprovalDecidedHR(toEmails []string, teamsEnabled bool, candName string, approved bool, dashURL string) []Message {
	outcome := "ถูกปฏิเสธ"
	title := "ผลการอนุมัติจ้าง: ไม่อนุมัติ"
	accent := emailtmpl.AccentDanger
	if approved {
		outcome = "ได้รับอนุมัติ (เข้าสู่ขั้นตอน Offer)"
		title = "ผลการอนุมัติจ้าง: อนุมัติ"
		accent = emailtmpl.AccentDefault
	}
	doc := emailtmpl.Doc{
		Title:      title,
		Paragraphs: []string{"คำขออนุมัติการจ้างได้ข้อสรุปแล้ว"},
		Details: []emailtmpl.DetailRow{
			{Label: "ผู้สมัคร", Value: fallback(candName, "ผู้สมัคร")},
			{Label: "ผล", Value: outcome},
		},
		CTA:    &emailtmpl.CTA{Label: "ดูรายละเอียด", URL: dashURL},
		Accent: accent,
	}
	return hrMessages(toEmails, teamsEnabled, doc)
}

// ApprovalEscalationHR reminds the responsible approvers that a chain level has
// passed its SLA without a decision.
func ApprovalEscalationHR(toEmails []string, teamsEnabled bool, candName, levelLabel, dashURL string) []Message {
	doc := emailtmpl.Doc{
		Title:      "เตือน: คำขออนุมัติจ้างเกินกำหนด (SLA)",
		Paragraphs: []string{"คำขออนุมัติการจ้างเกินกำหนดเวลาพิจารณาแล้ว กรุณาดำเนินการ"},
		Details: []emailtmpl.DetailRow{
			{Label: "ผู้สมัคร", Value: fallback(candName, "ผู้สมัคร")},
			{Label: "ขั้นอนุมัติ", Value: fallback(levelLabel, "-")},
		},
		CTA:    &emailtmpl.CTA{Label: "ดูรายการอนุมัติ", URL: dashURL},
		Accent: emailtmpl.AccentWarning,
	}
	return hrMessages(toEmails, teamsEnabled, doc)
}

// OnboardingDocUploadedHR notifies store HR that a hired candidate uploaded an
// onboarding document awaiting review (3.8). dashURL deep-links the application.
func OnboardingDocUploadedHR(toEmails []string, teamsEnabled bool, docType, dashURL string) []Message {
	doc := emailtmpl.Doc{
		Title:      "มีการอัปโหลดเอกสาร onboarding ใหม่",
		Paragraphs: []string{"ผู้สมัครที่ได้รับการจ้างได้อัปโหลดเอกสาร onboarding รอการตรวจสอบ"},
		Details:    []emailtmpl.DetailRow{{Label: "เอกสาร", Value: docTypeLabel(docType)}},
		CTA:        &emailtmpl.CTA{Label: "ดูรายละเอียด", URL: dashURL},
	}
	return hrMessages(toEmails, teamsEnabled, doc)
}

// OfferNegotiatedHR notifies store HR that a candidate countered their offer with a
// new figure and (optionally) a note, so HR can revise & re-send or end the
// negotiation. counterText is the pre-formatted counter amount (e.g. "25,000 บาท").
func OfferNegotiatedHR(toEmails []string, teamsEnabled bool, positionTitle, counterText, note, dashURL string) []Message {
	details := []emailtmpl.DetailRow{
		{Label: "ตำแหน่ง", Value: fallback(positionTitle, "-")},
		{Label: "ตัวเลขที่ผู้สมัครเสนอ", Value: fallback(counterText, "-")},
	}
	if note != "" {
		details = append(details, emailtmpl.DetailRow{Label: "หมายเหตุจากผู้สมัคร", Value: note})
	}
	doc := emailtmpl.Doc{
		Title:      "ผู้สมัครขอต่อรองข้อเสนอ",
		Paragraphs: []string{"ผู้สมัครได้ส่งตัวเลขที่ต้องการต่อรองกลับมา กรุณาพิจารณาปรับแก้แล้วส่งใหม่ หรือยุติการต่อรอง"},
		Details:    details,
		CTA:        &emailtmpl.CTA{Label: "ดูรายละเอียด", URL: dashURL},
	}
	return hrMessages(toEmails, teamsEnabled, doc)
}

// hrMessages fans a Doc out to one branded email Message per address plus an
// optional Teams Message. The email carries the rendered HTML; the Teams message
// carries only the plain body (Teams renders its own Adaptive Card from it).
func hrMessages(toEmails []string, teamsEnabled bool, doc emailtmpl.Doc) []Message {
	body := doc.PlainText()
	html := emailtmpl.Render(doc)
	msgs := make([]Message, 0, len(toEmails)+1)
	for _, addr := range toEmails {
		if addr == "" {
			continue
		}
		msgs = append(msgs, Message{Channel: ChannelEmail, Recipient: addr, Subject: doc.Title, Body: body, HTML: html})
	}
	if teamsEnabled {
		msgs = append(msgs, Message{Channel: ChannelTeams, Subject: doc.Title, Body: body})
	}
	return msgs
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
