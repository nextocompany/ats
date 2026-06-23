package dedup

import "testing"

func TestNameLooselyMatches(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
		note string
	}{
		// Should MATCH (must never falsely reject these).
		{"สมชาย ใจดี", "สมชาย ใจดี", true, "identical"},
		{"นาย สมชาย ใจดี", "สมชาย ใจดี", true, "honorific stripped"},
		{"สมชาย ใจดี", "ใจดี สมชาย", true, "name order swapped"},
		{"สมชาย ใจดี", "สมชาย", true, "first name only (account fuller)"},
		{"สมชาย", "สมชาย ใจดี", true, "resume fuller than account"},
		{"Somchai Jaidee", "somchai  jaidee", true, "case + extra space"},
		{"สมชาย ใจดี", "สมชัย ใจดี", true, "1-char OCR slip in a token"},
		{"Nattapong Sukjai", "Nattapong S.", true, "shared first name"},
		{"", "สมชาย", true, "empty account name → do not block"},
		{"สมชาย", "", true, "empty resume name → do not block"},

		// Should NOT match (clearly a different person → flag).
		{"สมชาย ใจดี", "ปิติพงษ์ มั่งมี", false, "completely different person"},
		{"Somchai Jaidee", "Wichai Thongdee", false, "different EN names"},
		{"อรทัย แสงเดือน", "ก้องภพ ทองสุข", false, "different TH names"},
	}
	for _, c := range cases {
		if got := NameLooselyMatches(c.a, c.b); got != c.want {
			t.Errorf("NameLooselyMatches(%q,%q)=%v want %v (%s)", c.a, c.b, got, c.want, c.note)
		}
	}
}
