package notify

import (
	"fmt"
	"time"

	"github.com/nexto/hr-ats/pkg/emailtmpl"
	// Embed the IANA tz database into the binary so time.LoadLocation("Asia/Bangkok")
	// works even on a minimal container image (the prod image is alpine without the
	// tzdata package). Without this the LoadLocation below would always error and
	// silently fall back to the fixed +07:00 zone.
	_ "time/tzdata"
)

// interviewInviteDoc builds the shared content for the AI pre-interview invite.
// The CTA carries a deep link to the portal interview page gated by the opaque
// access token. The token lives in the URL fragment (#), not the query string, so
// it is not sent to the server / proxies and stays out of access logs and
// referrers.
func interviewInviteDoc(fullName, portalBaseURL, token string) emailtmpl.Doc {
	return emailtmpl.Doc{
		Title:    "เชิญทำสัมภาษณ์ AI เบื้องต้น",
		Greeting: emailtmpl.Greeting(fullName),
		Paragraphs: []string{
			"คุณได้รับเชิญให้ทำสัมภาษณ์ AI เบื้องต้น ใช้เวลาประมาณ 5 นาที กรุณาเริ่มทำผ่านปุ่มด้านล่าง",
		},
		CTA: &emailtmpl.CTA{Label: "เริ่มสัมภาษณ์ AI", URL: fmt.Sprintf("%s/interview#token=%s", portalBaseURL, token)},
	}
}

// InterviewInviteMessage builds a candidate-facing LINE notification inviting the
// candidate to complete the AI pre-interview. Like StatusMessage it takes
// primitives (not domain structs) to keep this package decoupled, and returns a
// zero Message (empty Recipient) — which callers skip — when the candidate has no
// LINE handle or no token was issued.
func InterviewInviteMessage(lineUserID, fullName, portalBaseURL, token string) Message {
	if lineUserID == "" || token == "" {
		return Message{}
	}
	return Message{
		Channel:   ChannelLINE,
		Recipient: lineUserID,
		Subject:   "เชิญทำสัมภาษณ์ AI เบื้องต้น",
		Body:      interviewInviteDoc(fullName, portalBaseURL, token).PlainText(),
	}
}

// InterviewInviteEmailMessage is the email twin of InterviewInviteMessage, reaching
// candidates who applied with an email but no LINE handle (previously the AI invite
// was LINE-only). Reuses interviewInviteDoc so the channels never drift. Returns a
// zero Message when the address is empty or no token was issued.
func InterviewInviteEmailMessage(emailAddr, fullName, portalBaseURL, token string) Message {
	if emailAddr == "" || token == "" {
		return Message{}
	}
	doc := interviewInviteDoc(fullName, portalBaseURL, token)
	return Message{
		Channel:   ChannelEmail,
		Recipient: emailAddr,
		Subject:   "เชิญทำสัมภาษณ์ AI เบื้องต้น",
		Body:      doc.PlainText(),
		HTML:      emailtmpl.Render(doc),
	}
}

// InterviewScheduledMessage builds a candidate-facing LINE notification for a
// booked human interview, carrying the concrete appointment (Thai-formatted
// Bangkok date/time, round label, and either the onsite location or the online
// join link). Like the other builders it takes primitives and returns a zero
// Message (skipped by callers) when the candidate has no LINE handle.
func InterviewScheduledMessage(lineUserID, fullName string, roundNo, durationMin int, scheduledAt time.Time, mode, locationText, onlineJoinURL, portalBaseURL, token string) Message {
	if lineUserID == "" {
		return Message{}
	}
	return Message{
		Channel:   ChannelLINE,
		Recipient: lineUserID,
		Subject:   "นัดหมายสัมภาษณ์",
		Body:      interviewScheduledDoc(fullName, roundNo, durationMin, scheduledAt, mode, locationText, onlineJoinURL, portalBaseURL, token).PlainText(),
	}
}

// InterviewScheduledEmailMessage is the email twin of InterviewScheduledMessage,
// reusing the same Doc so the LINE and email copy never drift. Returns a zero
// Message when the address is empty.
func InterviewScheduledEmailMessage(emailAddr, fullName string, roundNo, durationMin int, scheduledAt time.Time, mode, locationText, onlineJoinURL, portalBaseURL, token string) Message {
	if emailAddr == "" {
		return Message{}
	}
	doc := interviewScheduledDoc(fullName, roundNo, durationMin, scheduledAt, mode, locationText, onlineJoinURL, portalBaseURL, token)
	return Message{
		Channel:   ChannelEmail,
		Recipient: emailAddr,
		Subject:   "นัดหมายสัมภาษณ์",
		Body:      doc.PlainText(),
		HTML:      emailtmpl.Render(doc),
	}
}

func interviewScheduledDoc(fullName string, roundNo, durationMin int, scheduledAt time.Time, mode, locationText, onlineJoinURL, portalBaseURL, token string) emailtmpl.Doc {
	lead := "นัดสัมภาษณ์ของคุณ"
	if roundNo > 1 {
		lead = fmt.Sprintf("นัดสัมภาษณ์ (รอบ %d) ของคุณ", roundNo)
	}
	dateVal := thaiDateTimeBangkok(scheduledAt)
	if durationMin > 0 {
		dateVal += fmt.Sprintf(" (ประมาณ %d นาที)", durationMin)
	}
	details := []emailtmpl.DetailRow{{Label: "📅 วันเวลา", Value: dateVal}}
	// mode is pre-validated to "onsite"/"online" at the schedule handler; an
	// unexpected value renders the onsite label (the safe default).
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
		Title:      "นัดหมายสัมภาษณ์",
		Greeting:   emailtmpl.Greeting(fullName),
		Paragraphs: []string{lead},
		Details:    details,
		CTA:        &emailtmpl.CTA{Label: "ดูรายละเอียดการนัดหมาย", URL: statusPath(portalBaseURL, token)},
	}
}

// thaiMonths and thaiDateTimeBangkok format a timestamp in Thai Buddhist-era long
// form with the time, e.g. "25 มิถุนายน 2569 เวลา 14:00 น.". The appointment time
// is parsed from a client RFC3339 string and may carry a UTC offset, so it is
// always converted to Asia/Bangkok first — otherwise the candidate would see a
// time off by 7 hours. Falls back to a fixed +07:00 zone if the OS tz database is
// unavailable (e.g. a minimal container) so the build never depends on tzdata.
var thaiMonths = [...]string{
	"มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน", "พฤษภาคม", "มิถุนายน",
	"กรกฎาคม", "สิงหาคม", "กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม",
}

func thaiDateTimeBangkok(t time.Time) string {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	t = t.In(loc)
	return fmt.Sprintf("%d %s %d เวลา %02d:%02d น.", t.Day(), thaiMonths[int(t.Month())-1], t.Year()+543, t.Hour(), t.Minute())
}
