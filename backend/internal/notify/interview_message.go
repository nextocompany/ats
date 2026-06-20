package notify

import (
	"fmt"
	"time"
	// Embed the IANA tz database into the binary so time.LoadLocation("Asia/Bangkok")
	// works even on a minimal container image (the prod image is alpine without the
	// tzdata package). Without this the LoadLocation below would always error and
	// silently fall back to the fixed +07:00 zone.
	_ "time/tzdata"
)

// InterviewInviteMessage builds a candidate-facing LINE notification inviting the
// candidate to complete the AI pre-interview. Like StatusMessage it takes
// primitives (not domain structs) to keep this package decoupled, and returns a
// zero Message (empty Recipient) — which callers skip — when the candidate has no
// LINE handle or no token was issued. The body carries a deep link to the portal
// interview page gated by the opaque access token.
func InterviewInviteMessage(lineUserID, fullName, portalBaseURL, token string) Message {
	if lineUserID == "" || token == "" {
		return Message{}
	}
	greeting := "สวัสดีค่ะ"
	if fullName != "" {
		greeting = "สวัสดีคุณ" + fullName
	}
	body := greeting +
		" คุณได้รับเชิญให้ทำสัมภาษณ์ AI เบื้องต้น ใช้เวลาประมาณ 5 นาที กรุณาทำผ่านลิงก์นี้ " +
		// Token lives in the URL fragment (#), not the query string, so it is not
		// sent to the server / proxies and stays out of access logs and referrers.
		fmt.Sprintf("%s/interview#token=%s", portalBaseURL, token)
	return Message{
		Channel:   ChannelLINE,
		Recipient: lineUserID,
		Subject:   "เชิญทำสัมภาษณ์ AI เบื้องต้น",
		Body:      body,
	}
}

// InterviewScheduledMessage builds a candidate-facing LINE notification for a
// booked human interview, carrying the concrete appointment (Thai-formatted
// Bangkok date/time, round label, and either the onsite location or the online
// join link). Like the other builders it takes primitives and returns a zero
// Message (skipped by callers) when the candidate has no LINE handle.
func InterviewScheduledMessage(lineUserID, fullName string, roundNo, durationMin int, scheduledAt time.Time, mode, locationText, onlineJoinURL, portalBaseURL string) Message {
	if lineUserID == "" {
		return Message{}
	}
	return Message{
		Channel:   ChannelLINE,
		Recipient: lineUserID,
		Subject:   "นัดหมายสัมภาษณ์",
		Body:      interviewScheduledBody(fullName, roundNo, durationMin, scheduledAt, mode, locationText, onlineJoinURL, portalBaseURL),
	}
}

// InterviewScheduledEmailMessage is the email twin of InterviewScheduledMessage,
// reusing the same body so the LINE and email copy never drift. Returns a zero
// Message when the address is empty.
func InterviewScheduledEmailMessage(emailAddr, fullName string, roundNo, durationMin int, scheduledAt time.Time, mode, locationText, onlineJoinURL, portalBaseURL string) Message {
	if emailAddr == "" {
		return Message{}
	}
	return Message{
		Channel:   ChannelEmail,
		Recipient: emailAddr,
		Subject:   "นัดหมายสัมภาษณ์",
		Body:      interviewScheduledBody(fullName, roundNo, durationMin, scheduledAt, mode, locationText, onlineJoinURL, portalBaseURL),
	}
}

func interviewScheduledBody(fullName string, roundNo, durationMin int, scheduledAt time.Time, mode, locationText, onlineJoinURL, portalBaseURL string) string {
	greeting := "สวัสดีค่ะ"
	if fullName != "" {
		greeting = "สวัสดีคุณ" + fullName
	}
	round := ""
	if roundNo > 1 {
		round = fmt.Sprintf(" (รอบ %d)", roundNo)
	}
	body := greeting + fmt.Sprintf(" นัดสัมภาษณ์%s ของคุณ\n📅 %s", round, thaiDateTimeBangkok(scheduledAt))
	if durationMin > 0 {
		body += fmt.Sprintf(" (ประมาณ %d นาที)", durationMin)
	}
	// mode is pre-validated to "onsite"/"online" at the schedule handler; an
	// unexpected value renders the onsite label (the safe default).
	if mode == "online" {
		body += "\n💻 สัมภาษณ์ออนไลน์"
		if onlineJoinURL != "" {
			body += " — เข้าร่วมที่ " + onlineJoinURL
		}
	} else {
		body += "\n📍 สัมภาษณ์ที่สถานที่"
		if locationText != "" {
			body += ": " + locationText
		}
	}
	return body + fmt.Sprintf("\nดูรายละเอียดได้ที่ %s/status", portalBaseURL)
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
