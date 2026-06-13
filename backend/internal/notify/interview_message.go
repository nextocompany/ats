package notify

import "fmt"

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
