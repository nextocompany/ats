package scoring

import (
	"context"

	"github.com/nexto/hr-ats/internal/ai"
)

// mockLLM returns deterministic qualitative output for dev/CI.
type mockLLM struct{}

func (mockLLM) evaluate(_ context.Context, p ai.Profile, jd JD) (LLMPart, error) {
	// Deterministic skills score from keyword overlap so CI is reproducible.
	skills := 10
	if overlap := keywordOverlap(p.Skills, jd.Keywords); overlap > 0 {
		skills = clamp(10+overlap*3, 0, 20)
	}
	return LLMPart{
		SkillsScore: skills,
		Strengths: []string{
			"มีประสบการณ์ตรงกับตำแหน่งงาน",
			"ทักษะตรงกับคุณสมบัติที่ต้องการ",
			"มีความพร้อมในการเริ่มงาน",
		},
		RedFlags:           nil,
		SuggestedPositions: nil,
	}, nil
}

func keywordOverlap(skills, keywords []string) int {
	set := make(map[string]struct{}, len(keywords))
	for _, k := range keywords {
		set[k] = struct{}{}
	}
	count := 0
	for _, s := range skills {
		if _, ok := set[s]; ok {
			count++
		}
	}
	return count
}
