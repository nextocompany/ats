package fit

import "context"

// mockSummarizer returns a deterministic Thai analysis so local/dev/CI runs work
// without Azure credentials. With no positions it reports a clean "no fit"; with
// positions it recommends the first one or two.
type mockSummarizer struct{}

func (mockSummarizer) Summarize(_ context.Context, in Inputs) (Analysis, error) {
	a := Analysis{
		Strengths: []string{"มีประสบการณ์ตรงกับงานค้าปลีก", "สื่อสารและทำงานเป็นทีมได้ดี"},
		Concerns:  []string{"ควรประเมินทักษะเชิงลึกเพิ่มเติมในการสัมภาษณ์รอบถัดไป"},
	}

	if len(in.Positions) == 0 {
		a.OverallFit = OverallNone
		a.Summary = "ยังไม่มีตำแหน่งใน Master JD ให้เปรียบเทียบ จึงไม่สามารถแนะนำตำแหน่งที่เหมาะสมได้"
		a.NoMatchReason = "ไม่พบตำแหน่งที่เปิดอยู่ในระบบสำหรับการเปรียบเทียบ"
		a.Recommended = []RecommendedPosition{}
		return a, nil
	}

	limit := 2
	if len(in.Positions) < limit {
		limit = len(in.Positions)
	}
	for i := 0; i < limit; i++ {
		p := in.Positions[i]
		a.Recommended = append(a.Recommended, RecommendedPosition{
			PositionID: p.ID,
			Title:      p.Title,
			FitScore:   80 - i*12,
			Reasons:    []string{"คุณสมบัติและประสบการณ์สอดคล้องกับหน้าที่ความรับผิดชอบของตำแหน่งนี้"},
		})
	}
	a.OverallFit = OverallModerate
	a.Summary = "ผู้สมัครมีคุณสมบัติเหมาะสมกับบางตำแหน่งในองค์กร โดยเฉพาะตำแหน่งที่แนะนำด้านล่าง"
	a.Model = "mock"
	return a, nil
}
