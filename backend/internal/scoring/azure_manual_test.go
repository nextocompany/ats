package scoring

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"net/http"

	"github.com/nexto/hr-ats/internal/ai"
)

// TestManual_RealLLM_StrengthsNoNegations hits the real Azure OpenAI deployment to
// verify the UAT #4 fix: the scorer must never list a gap/absence as a "strength".
// Skipped unless RUN_LLM_MANUAL=1 and AZURE_OPENAI_* are set (so CI never calls the
// network). Run with:
//
//	RUN_LLM_MANUAL=1 AZURE_OPENAI_ENDPOINT=... AZURE_OPENAI_KEY=... AZURE_OPENAI_DEPLOYMENT=... \
//	  go test ./internal/scoring/ -run TestManual_RealLLM -v
func TestManual_RealLLM_StrengthsNoNegations(t *testing.T) {
	if os.Getenv("RUN_LLM_MANUAL") != "1" {
		t.Skip("set RUN_LLM_MANUAL=1 to run the real-LLM manual check")
	}
	a := azureLLM{
		endpoint:   strings.TrimRight(os.Getenv("AZURE_OPENAI_ENDPOINT"), "/"),
		key:        os.Getenv("AZURE_OPENAI_KEY"),
		deployment: os.Getenv("AZURE_OPENAI_DEPLOYMENT"),
		http:       &http.Client{Timeout: 60 * time.Second},
	}
	if a.endpoint == "" || a.key == "" || a.deployment == "" {
		t.Fatal("AZURE_OPENAI_ENDPOINT/KEY/DEPLOYMENT required")
	}

	// Negation markers that must never appear inside a "strength".
	negations := []string{"ไม่มี", "ขาด", "ไม่ได้", "ไม่ตรง", "ไม่เคย", "ไม่เพียงพอ", "ไม่สอดคล้อง"}

	bakeryJD := JD{
		Title:            "เบเกอร์ / พนักงานเบเกอรี่",
		Responsibilities: "ผลิตและอบขนมปังเบเกอรี่ตามสูตร ควบคุมคุณภาพและความสะอาด จัดเตรียมวัตถุดิบ",
		Qualifications:   "มีประสบการณ์งานเบเกอรี่/ทำขนมอบอย่างน้อย 1 ปี ทำงานเป็นกะได้",
		Keywords:         []string{"เบเกอรี่", "ขนมอบ, baking", "ครัว"},
	}

	cases := []struct {
		name    string
		profile ai.Profile
	}{
		{
			name: "weak-mismatch (the bakery bug)",
			profile: ai.Profile{
				Personal:   ai.Personal{Name: "สมหญิง รักงาน", Age: 27},
				Experience: []ai.Experience{{Company: "บริษัทบัญชี ABC", Position: "พนักงานบัญชี", DurationMonths: 36, Description: "ทำบัญชีรายรับรายจ่าย ปิดงบ ภาษี"}},
				Education:  []ai.Education{{Degree: "ปริญญาตรี", Major: "การบัญชี", Institution: "ม.ราชภัฏ", Year: 2562}},
				Skills:     []string{"Excel", "บัญชี", "ภาษี"},
			},
		},
		{
			name: "borderline (transferable but missing core req)",
			profile: ai.Profile{
				Personal:   ai.Personal{Name: "สมศักดิ์ ขยัน", Age: 24},
				Experience: []ai.Experience{{Company: "ร้านอาหารตามสั่ง", Position: "ผู้ช่วยกุ๊ก", DurationMonths: 24, Description: "เตรียมวัตถุดิบ ดูแลความสะอาดในครัว ทำงานเป็นทีม"}},
				Education:  []ai.Education{{Degree: "ม.6", Major: "ทั่วไป", Institution: "โรงเรียนมัธยม", Year: 2561}},
				Skills:     []string{"เตรียมวัตถุดิบ", "ความสะอาดในครัว", "ทำงานเป็นทีม"},
			},
		},
		{
			name: "messy-ocr (garbled, partial fields, buzzwords)",
			profile: ai.Profile{
				Personal:   ai.Personal{Name: "x สมช? ย", Age: 0},
				Experience: []ai.Experience{{Company: "—", Position: "พนกงาน", DurationMonths: 0, Description: "ทำงานทวไป รบผดชอบ มความรบผดชอบสง teamwork passionate"}},
				Education:  []ai.Education{{Degree: "", Major: "", Institution: "??", Year: 0}},
				Skills:     []string{"Microsoft Word", "passionate", "fast learner"},
			},
		},
		{
			name: "strong-fit (regression guard)",
			profile: ai.Profile{
				Personal:   ai.Personal{Name: "สมชาย ใจดี", Age: 30},
				Experience: []ai.Experience{{Company: "ร้านเบเกอรี่ Sweet", Position: "เบเกอร์", DurationMonths: 48, Description: "อบขนมปัง เค้ก ครัวซองต์ ควบคุมสูตรและคุณภาพ"}},
				Education:  []ai.Education{{Degree: "ปวส.", Major: "อาหารและโภชนาการ", Institution: "วิทยาลัยอาชีวศึกษา", Year: 2560}},
				Skills:     []string{"เบเกอรี่", "ขนมอบ", "ควบคุมคุณภาพ", "ทำงานเป็นกะ"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := a.evaluate(context.Background(), tc.profile, bakeryJD)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			t.Logf("skills_score=%d", out.SkillsScore)
			t.Logf("strengths=%v", out.Strengths)
			t.Logf("red_flags=%v", out.RedFlags)
			for _, s := range out.Strengths {
				for _, neg := range negations {
					if strings.Contains(s, neg) {
						t.Errorf("strength contains a negation %q (gap listed as strength): %q", neg, s)
					}
				}
			}
		})
	}
}
