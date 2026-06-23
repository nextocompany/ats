package notify

import "fmt"

// StatusMessage builds a candidate-facing LINE notification for a status
// transition. It takes primitives (not domain structs) to keep this package
// decoupled from applications/candidates.
//
// Returns a zero Message (empty Recipient) — which callers skip — when:
//   - the status is not candidate-notifiable (only shortlisted/interview/hired), or
//   - the candidate has no LINE handle (legacy/demo, or never logged in via LINE).
//
// Rejections are intentionally NOT pushed: a LINE rejection is a poor experience,
// and the public status page already carries that message.
func StatusMessage(lineUserID, fullName, status, portalBaseURL string) Message {
	if lineUserID == "" {
		return Message{}
	}
	body, ok := statusBody(fullName, status, portalBaseURL)
	if !ok {
		return Message{}
	}
	return Message{
		Channel:   ChannelLINE,
		Recipient: lineUserID,
		Subject:   "อัปเดตสถานะใบสมัคร",
		Body:      body,
	}
}

// StatusEmailMessage builds the email equivalent of StatusMessage for the same
// candidate-notifiable status set, reusing statusBody so the LINE and email copy
// never drift. Returns a zero Message (empty Recipient) when the address is empty
// or the status is not notifiable.
func StatusEmailMessage(emailAddr, fullName, status, portalBaseURL string) Message {
	if emailAddr == "" {
		return Message{}
	}
	body, ok := statusBody(fullName, status, portalBaseURL)
	if !ok {
		return Message{}
	}
	return Message{
		Channel:   ChannelEmail,
		Recipient: emailAddr,
		Subject:   "อัปเดตสถานะใบสมัคร",
		Body:      body,
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
		Body:      documentReviewedBody(fullName, docType, approved, reason, portalBaseURL),
	}
}

// DocumentReviewedEmailMessage is the email equivalent of DocumentReviewedMessage,
// reusing the same body so the LINE and email copy never drift. Returns a zero
// Message when the address is empty.
func DocumentReviewedEmailMessage(emailAddr, fullName, docType string, approved bool, reason, portalBaseURL string) Message {
	if emailAddr == "" {
		return Message{}
	}
	return Message{
		Channel:   ChannelEmail,
		Recipient: emailAddr,
		Subject:   "อัปเดตเอกสาร onboarding",
		Body:      documentReviewedBody(fullName, docType, approved, reason, portalBaseURL),
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
		Body:      invalidResumeBody(fullName, portalBaseURL),
	}
}

// InvalidResumeEmailMessage is the email equivalent of InvalidResumeMessage,
// reusing the same body so the LINE and email copy never drift. Returns a zero
// Message when the address is empty.
func InvalidResumeEmailMessage(emailAddr, fullName, portalBaseURL string) Message {
	if emailAddr == "" {
		return Message{}
	}
	return Message{
		Channel:   ChannelEmail,
		Recipient: emailAddr,
		Subject:   "กรุณาอัปโหลดเรซูเม่อีกครั้ง",
		Body:      invalidResumeBody(fullName, portalBaseURL),
	}
}

func invalidResumeBody(fullName, portalBaseURL string) string {
	greeting := "สวัสดีค่ะ"
	if fullName != "" {
		greeting = "สวัสดีคุณ" + fullName
	}
	return greeting +
		" ไฟล์ที่คุณอัปโหลดอาจไม่ใช่เรซูเม่/CV จึงยังไม่สามารถพิจารณาใบสมัครของคุณได้ " +
		"กรุณาตรวจสอบและอัปโหลดไฟล์เรซูเม่ของคุณอีกครั้ง" +
		fmt.Sprintf(" ที่ %s/status", portalBaseURL)
}

func documentReviewedBody(fullName, docType string, approved bool, reason, portalBaseURL string) string {
	greeting := "สวัสดีค่ะ"
	if fullName != "" {
		greeting = "สวัสดีคุณ" + fullName
	}
	label := docTypeLabel(docType)
	link := fmt.Sprintf(" ดูรายละเอียดได้ที่ %s/account", portalBaseURL)
	if approved {
		return greeting + fmt.Sprintf(" เอกสาร \"%s\" ของคุณได้รับการอนุมัติแล้ว", label) + link
	}
	return greeting + fmt.Sprintf(" เอกสาร \"%s\" ของคุณถูกตีกลับ เหตุผล: %s กรุณาอัปโหลดใหม่", label, fallback(reason, "-")) + link
}

func statusBody(fullName, status, portalBaseURL string) (string, bool) {
	greeting := "สวัสดีค่ะ"
	if fullName != "" {
		greeting = "สวัสดีคุณ" + fullName
	}
	link := fmt.Sprintf(" ตรวจสอบสถานะได้ที่ %s/status", portalBaseURL)
	switch status {
	case "shortlisted":
		return greeting + " ใบสมัครของคุณผ่านการคัดกรองเบื้องต้นและเข้าสู่รอบพิจารณา ทีม HR จะติดต่อกลับ" + link, true
	case "interview":
		return greeting + " คุณได้รับเชิญเข้าสัมภาษณ์ ทีมงานจะติดต่อเพื่อนัดหมายเร็ว ๆ นี้" + link, true
	case "offer":
		return greeting + fmt.Sprintf(" คุณได้รับข้อเสนอการจ้างงาน! เข้าสู่ระบบเพื่อดูรายละเอียดและตอบรับได้ที่ %s/offers", portalBaseURL), true
	case "hired":
		return greeting + fmt.Sprintf(" ยินดีด้วย! คุณได้รับการคัดเลือก กรุณาอัปโหลดเอกสารเริ่มงานของคุณที่ %s/account ทีม HR จะติดต่อกลับเรื่องวันเริ่มงาน", portalBaseURL), true
	default:
		return "", false
	}
}
