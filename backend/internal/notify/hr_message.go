package notify

import "fmt"

// HR-facing notification builders. Each returns the set of Messages to dispatch:
// one ChannelEmail per recipient address plus, when teamsEnabled, one ChannelTeams
// message (its Recipient is empty — the webhook is the target). Primitives only, to
// keep this package decoupled from applications/candidates.

// NewScoredHR notifies store HR that a new candidate has been screened + scored and
// assigned to their store. dashURL is the dashboard deep link to the application.
func NewScoredHR(toEmails []string, teamsEnabled bool, candName, positionTitle string, score int, dashURL string) []Message {
	subject := "ผู้สมัครใหม่ผ่านการคัดกรอง"
	body := fmt.Sprintf(
		"มีผู้สมัครใหม่ผ่านการคัดกรองและถูกจัดให้สาขาของคุณ\nผู้สมัคร: %s\nตำแหน่ง: %s\nคะแนน AI: %d/100\nดูรายละเอียด: %s",
		fallback(candName, "ผู้สมัคร"), fallback(positionTitle, "-"), score, dashURL,
	)
	return hrMessages(toEmails, teamsEnabled, subject, body)
}

// FeedbackRecordedHR notifies store HR that an interviewer recorded interview
// feedback for a candidate.
func FeedbackRecordedHR(toEmails []string, teamsEnabled bool, candName, positionTitle, interviewer, recommendation, dashURL string) []Message {
	subject := "บันทึกผลสัมภาษณ์ใหม่"
	body := fmt.Sprintf(
		"มีการบันทึกผลสัมภาษณ์ใหม่\nผู้สมัคร: %s\nตำแหน่ง: %s\nผู้สัมภาษณ์: %s\nผลสรุป: %s\nดูรายละเอียด: %s",
		fallback(candName, "ผู้สมัคร"), fallback(positionTitle, "-"),
		fallback(interviewer, "-"), fallback(recommendation, "-"), dashURL,
	)
	return hrMessages(toEmails, teamsEnabled, subject, body)
}

// ShortlistReadyLM notifies a store's Line Manager(s) that a candidate has been
// shortlisted and is awaiting their Top-5 review. dashURL deep-links the shortlist.
func ShortlistReadyLM(lmEmails []string, teamsEnabled bool, candName, positionTitle, dashURL string) []Message {
	subject := "มีผู้สมัครรอการพิจารณา (Shortlist)"
	body := fmt.Sprintf(
		"มีผู้สมัครถูกคัดเข้า shortlist รอการพิจารณาของผู้จัดการสาขา\nผู้สมัคร: %s\nตำแหน่ง: %s\nดูรายชื่อคัดสรร: %s",
		fallback(candName, "ผู้สมัคร"), fallback(positionTitle, "-"), dashURL,
	)
	return hrMessages(lmEmails, teamsEnabled, subject, body)
}

// ApprovalPendingHR notifies the approvers at a newly active chain level that a
// hire approval is awaiting their decision. levelLabel is e.g. "HR Manager".
func ApprovalPendingHR(toEmails []string, teamsEnabled bool, candName, levelLabel, dashURL string) []Message {
	subject := "มีคำขออนุมัติจ้างรอการพิจารณา"
	body := fmt.Sprintf(
		"มีคำขออนุมัติการจ้างรอการอนุมัติของคุณ\nผู้สมัคร: %s\nขั้นอนุมัติ: %s\nดูรายการอนุมัติ: %s",
		fallback(candName, "ผู้สมัคร"), fallback(levelLabel, "-"), dashURL,
	)
	return hrMessages(toEmails, teamsEnabled, subject, body)
}

// ApprovalDecidedHR notifies HR of the final outcome of an approval chain.
func ApprovalDecidedHR(toEmails []string, teamsEnabled bool, candName string, approved bool, dashURL string) []Message {
	outcome := "ถูกปฏิเสธ"
	subject := "ผลการอนุมัติจ้าง: ไม่อนุมัติ"
	if approved {
		outcome = "ได้รับอนุมัติ (เข้าสู่ขั้นตอน Offer)"
		subject = "ผลการอนุมัติจ้าง: อนุมัติ"
	}
	body := fmt.Sprintf(
		"คำขออนุมัติการจ้างได้ข้อสรุปแล้ว\nผู้สมัคร: %s\nผล: %s\nดูรายละเอียด: %s",
		fallback(candName, "ผู้สมัคร"), outcome, dashURL,
	)
	return hrMessages(toEmails, teamsEnabled, subject, body)
}

// ApprovalEscalationHR reminds the responsible approvers that a chain level has
// passed its SLA without a decision.
func ApprovalEscalationHR(toEmails []string, teamsEnabled bool, candName, levelLabel, dashURL string) []Message {
	subject := "เตือน: คำขออนุมัติจ้างเกินกำหนด (SLA)"
	body := fmt.Sprintf(
		"คำขออนุมัติการจ้างเกินกำหนดเวลาพิจารณาแล้ว กรุณาดำเนินการ\nผู้สมัคร: %s\nขั้นอนุมัติ: %s\nดูรายการอนุมัติ: %s",
		fallback(candName, "ผู้สมัคร"), fallback(levelLabel, "-"), dashURL,
	)
	return hrMessages(toEmails, teamsEnabled, subject, body)
}

// hrMessages fans a subject/body out to one email Message per address plus an
// optional Teams Message.
func hrMessages(toEmails []string, teamsEnabled bool, subject, body string) []Message {
	msgs := make([]Message, 0, len(toEmails)+1)
	for _, addr := range toEmails {
		if addr == "" {
			continue
		}
		msgs = append(msgs, Message{Channel: ChannelEmail, Recipient: addr, Subject: subject, Body: body})
	}
	if teamsEnabled {
		msgs = append(msgs, Message{Channel: ChannelTeams, Subject: subject, Body: body})
	}
	return msgs
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
