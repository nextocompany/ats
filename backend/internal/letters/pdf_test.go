package letters

import (
	"bytes"
	"testing"
	"time"
)

func TestRender_Interview(t *testing.T) {
	r := NewRenderer("CP AXTRA")
	out, err := r.Render(LetterData{
		Type:          TypeInterview,
		CandidateName: "สมชาย ใจดี",
		PositionTitle: "พนักงานขาย",
		StoreName:     "สาขารัชดา",
		IssuedDate:    time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC),
		Interview: &InterviewDetails{
			ScheduledAt: time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC),
			DurationMin: 45,
			Mode:        "onsite",
			Location:    "อาคารสำนักงานใหญ่ ชั้น 5",
		},
	})
	if err != nil {
		t.Fatalf("render interview: %v", err)
	}
	assertPDF(t, out)
}

func TestRender_Offer(t *testing.T) {
	r := NewRenderer("CP AXTRA")
	out, err := r.Render(LetterData{
		Type:          TypeOffer,
		CandidateName: "สุดา รักงาน",
		PositionTitle: "ผู้จัดการสาขา",
		StoreName:     "สาขาพระราม 9",
		Offer: &OfferDetails{
			Salary:    35000,
			StartDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			Terms:     "ทดลองงาน 119 วัน ประกันสังคมตามกฎหมาย",
		},
	})
	if err != nil {
		t.Fatalf("render offer: %v", err)
	}
	assertPDF(t, out)
}

func TestRender_UnknownType(t *testing.T) {
	if _, err := NewRenderer("X").Render(LetterData{Type: "bogus"}); err == nil {
		t.Fatal("expected error for unknown letter type")
	}
}

func TestHumanizeTHB(t *testing.T) {
	cases := map[float64]string{0: "0", 999: "999", 1000: "1,000", 35000: "35,000", 1234567: "1,234,567", -1000: "-1,000"}
	for in, want := range cases {
		if got := humanizeTHB(in); got != want {
			t.Fatalf("humanizeTHB(%v)=%q want %q", in, got, want)
		}
	}
}

func assertPDF(t *testing.T, out []byte) {
	t.Helper()
	if len(out) == 0 {
		t.Fatal("empty PDF")
	}
	if !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatalf("not a PDF (prefix %q)", out[:min(8, len(out))])
	}
}
