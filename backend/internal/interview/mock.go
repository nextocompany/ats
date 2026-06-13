package interview

import "context"

// mockInterviewer is the deterministic, network-free default used by local dev
// and CI (no Azure credentials required). It walks a fixed Thai question script
// and returns a fixed evaluation, mirroring scoring.mockLLM.
type mockInterviewer struct{}

// mockQuestions is the canned adaptive-feeling script. The interview ends when
// the script is exhausted or MaxTurns is reached (whichever comes first).
var mockQuestions = []string{
	"สวัสดีค่ะ ขอบคุณที่สนใจร่วมงานกับเรานะคะ เล่าประสบการณ์การทำงานที่ผ่านมาให้ฟังหน่อยได้ไหมคะ",
	"เคยรับมือกับลูกค้าที่ไม่พอใจหรือสถานการณ์ยาก ๆ อย่างไรบ้างคะ",
	"อะไรคือจุดแข็งที่คุณคิดว่าเหมาะกับตำแหน่งนี้มากที่สุดคะ",
	"คุณสะดวกเริ่มงานได้เมื่อไหร่ และทำงานเป็นกะหรือวันหยุดได้ไหมคะ",
	"มีอะไรอยากสอบถามเกี่ยวกับตำแหน่งหรือบริษัทเพิ่มเติมไหมคะ",
}

func (mockInterviewer) NextTurn(_ context.Context, ic InterviewContext, history []Turn) (string, bool, error) {
	asked := 0
	for _, t := range history {
		if t.Role == RoleAssistant {
			asked++
		}
	}
	maxTurns := ic.MaxTurns
	if maxTurns <= 0 || maxTurns > len(mockQuestions) {
		maxTurns = len(mockQuestions)
	}
	if asked >= maxTurns {
		return "ขอบคุณสำหรับการพูดคุยในวันนี้นะคะ ทีม HR จะติดต่อกลับเร็ว ๆ นี้ค่ะ", true, nil
	}
	q := mockQuestions[asked%len(mockQuestions)]
	// The final question also closes the interview.
	done := asked+1 >= maxTurns
	return q, done, nil
}

func (mockInterviewer) Evaluate(_ context.Context, _ InterviewContext, _ []Turn) (Evaluation, error) {
	return Evaluation{
		Score:          75,
		Recommendation: RecPositive,
		Strengths: []string{
			"สื่อสารชัดเจนและมีทัศนคติเชิงบวก",
			"มีประสบการณ์ตรงกับตำแหน่งงาน",
		},
		Concerns: []string{
			"ควรยืนยันความพร้อมเรื่องวันเวลาทำงานอีกครั้ง",
		},
		Summary: "ผู้สมัครตอบคำถามได้ดี มีความเหมาะสมกับตำแหน่งในระดับที่น่าพิจารณาต่อ",
	}, nil
}
