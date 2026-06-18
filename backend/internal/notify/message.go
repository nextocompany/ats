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
		return greeting + " ยินดีด้วย! คุณได้รับการคัดเลือก ทีม HR จะติดต่อเรื่องการเริ่มงาน" + link, true
	default:
		return "", false
	}
}
