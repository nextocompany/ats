package fit

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestManual_RealLLM_FitStrengthsNoNegations hits the real Azure OpenAI deployment
// to verify the UAT #4 fix for the cross-org fit analysis: "strengths" (จุดเด่น)
// must never carry a gap/absence. Skipped unless RUN_LLM_MANUAL=1. See the scoring
// package's manual test for the run invocation.
func TestManual_RealLLM_FitStrengthsNoNegations(t *testing.T) {
	if os.Getenv("RUN_LLM_MANUAL") != "1" {
		t.Skip("set RUN_LLM_MANUAL=1 to run the real-LLM manual check")
	}
	a := azureSummarizer{
		endpoint:   strings.TrimRight(os.Getenv("AZURE_OPENAI_ENDPOINT"), "/"),
		key:        os.Getenv("AZURE_OPENAI_KEY"),
		deployment: os.Getenv("AZURE_OPENAI_DEPLOYMENT"),
		http:       &http.Client{Timeout: 60 * time.Second},
	}
	if a.endpoint == "" || a.key == "" || a.deployment == "" {
		t.Fatal("AZURE_OPENAI_ENDPOINT/KEY/DEPLOYMENT required")
	}

	negations := []string{"ไม่มี", "ขาด", "ไม่ได้", "ไม่ตรง", "ไม่เคย", "ไม่เพียงพอ", "ไม่สอดคล้อง"}

	catalogue := []PositionCard{
		{ID: uuid.New(), Title: "พนักงานบัญชี", Responsibilities: "บันทึกบัญชี ปิดงบ ยื่นภาษี", Qualifications: "ปริญญาตรีบัญชี มีประสบการณ์บัญชี"},
		{ID: uuid.New(), Title: "เบเกอร์", Responsibilities: "อบขนมปังและเบเกอรี่ตามสูตร", Qualifications: "ประสบการณ์เบเกอรี่ 1 ปี"},
		{ID: uuid.New(), Title: "พนักงานคลังสินค้า", Responsibilities: "รับ-จ่ายสินค้า ตรวจนับสต็อก", Qualifications: "ทำงานเป็นกะได้ แข็งแรง"},
	}
	score := func(v float64) *float64 { return &v }

	cases := []struct {
		name string
		in   Inputs
	}{
		{
			name: "mismatch-vs-applied (bakery applicant, accounting background)",
			in: Inputs{
				CandidateName:     "สมหญิง รักงาน",
				ScreeningScore:    score(28),
				ScreeningSummary:  "มีประสบการณ์งานบัญชี 3 ปี\nใช้ Excel และระบบบัญชีได้ดี",
				ScreeningRedFlags: "ไม่มีประสบการณ์งานเบเกอรี่; ทำงานเป็นกะไม่ได้",
				Positions:         catalogue,
			},
		},
		{
			name: "borderline (transferable kitchen, no bakery)",
			in: Inputs{
				CandidateName:     "สมศักดิ์ ขยัน",
				ScreeningScore:    score(45),
				ScreeningSummary:  "เคยเป็นผู้ช่วยกุ๊ก เตรียมวัตถุดิบและดูแลความสะอาดในครัว\nทำงานเป็นทีมได้ดี",
				ScreeningRedFlags: "ไม่มีประสบการณ์เบเกอรี่โดยตรง",
				Positions:         catalogue,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := a.Summarize(context.Background(), tc.in)
			if err != nil {
				t.Fatalf("summarize: %v", err)
			}
			t.Logf("overall_fit=%s", out.OverallFit)
			t.Logf("strengths=%v", out.Strengths)
			t.Logf("concerns=%v", out.Concerns)
			for _, r := range out.Recommended {
				t.Logf("recommended: %s (%d) reasons=%v", r.Title, r.FitScore, r.Reasons)
			}
			for _, s := range out.Strengths {
				for _, neg := range negations {
					if strings.Contains(s, neg) {
						t.Errorf("fit strength contains negation %q (gap as strength): %q", neg, s)
					}
				}
			}
		})
	}
}
