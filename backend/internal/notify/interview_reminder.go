package notify

import (
	"fmt"
	"time"

	"github.com/nexto/hr-ats/pkg/emailtmpl"
)

// InterviewReminderMessage builds a candidate-facing LINE reminder sent ~1 day
// before a booked human interview. It mirrors InterviewScheduledMessage (same
// appointment details, Thai Bangkok date/time, onsite location or online link)
// but with reminder copy. Returns a zero Message (skipped by callers) when the
// candidate has no LINE handle.
func InterviewReminderMessage(lineUserID, fullName string, roundNo, durationMin int, scheduledAt time.Time, mode, locationText, onlineJoinURL, portalBaseURL, token string) Message {
	if lineUserID == "" {
		return Message{}
	}
	return Message{
		Channel:   ChannelLINE,
		Recipient: lineUserID,
		Subject:   "เตือนความจำ: นัดสัมภาษณ์พรุ่งนี้",
		Body:      interviewReminderDoc(fullName, roundNo, durationMin, scheduledAt, mode, locationText, onlineJoinURL, portalBaseURL, token).PlainText(),
	}
}

// InterviewReminderEmailMessage is the email twin of InterviewReminderMessage,
// reusing the same Doc so the LINE and email copy never drift. Returns a zero
// Message when the address is empty.
func InterviewReminderEmailMessage(emailAddr, fullName string, roundNo, durationMin int, scheduledAt time.Time, mode, locationText, onlineJoinURL, portalBaseURL, token string) Message {
	if emailAddr == "" {
		return Message{}
	}
	doc := interviewReminderDoc(fullName, roundNo, durationMin, scheduledAt, mode, locationText, onlineJoinURL, portalBaseURL, token)
	return Message{
		Channel:   ChannelEmail,
		Recipient: emailAddr,
		Subject:   "เตือนความจำ: นัดสัมภาษณ์พรุ่งนี้",
		Body:      doc.PlainText(),
		HTML:      emailtmpl.Render(doc),
	}
}

func interviewReminderDoc(fullName string, roundNo, durationMin int, scheduledAt time.Time, mode, locationText, onlineJoinURL, portalBaseURL, token string) emailtmpl.Doc {
	lead := "เตือนความจำ: พรุ่งนี้คุณมีนัดสัมภาษณ์"
	if roundNo > 1 {
		lead = fmt.Sprintf("เตือนความจำ: พรุ่งนี้คุณมีนัดสัมภาษณ์ (รอบ %d)", roundNo)
	}
	dateVal := thaiDateTimeBangkok(scheduledAt)
	if durationMin > 0 {
		dateVal += fmt.Sprintf(" (ประมาณ %d นาที)", durationMin)
	}
	details := []emailtmpl.DetailRow{{Label: "📅 วันเวลา", Value: dateVal}}
	// mode is "onsite"/"online"; an unexpected value renders the onsite label.
	if mode == "online" {
		details = append(details, emailtmpl.DetailRow{Label: "💻 รูปแบบ", Value: "สัมภาษณ์ออนไลน์"})
		if onlineJoinURL != "" {
			details = append(details, emailtmpl.DetailRow{Label: "ลิงก์เข้าร่วม", Value: onlineJoinURL})
		}
	} else {
		details = append(details, emailtmpl.DetailRow{Label: "📍 รูปแบบ", Value: "สัมภาษณ์ที่สถานที่"})
		if locationText != "" {
			details = append(details, emailtmpl.DetailRow{Label: "สถานที่", Value: locationText})
		}
	}
	return emailtmpl.Doc{
		Title:      "เตือนความจำ: นัดสัมภาษณ์พรุ่งนี้",
		Greeting:   emailtmpl.Greeting(fullName),
		Paragraphs: []string{lead, "กรุณาเตรียมตัวและมาตามนัดหมายนะคะ"},
		Details:    details,
		CTA:        &emailtmpl.CTA{Label: "ดูรายละเอียดการนัดหมาย", URL: statusPath(portalBaseURL, token)},
	}
}
