package reengage

import (
	"testing"

	"github.com/nexto/hr-ats/internal/notify"
)

func TestPickChannel(t *testing.T) {
	tests := []struct {
		name      string
		target    Target
		wantChan  string
		wantRecip string
	}{
		{"prefers LINE when line id present", Target{LineUserID: "U-1", Email: "a@b.co", Phone: "08"}, notify.ChannelLINE, "U-1"},
		{"falls back to email", Target{Email: "a@b.co", Phone: "08"}, notify.ChannelEmail, "a@b.co"},
		{"phone is not a LINE handle → no channel", Target{Phone: "0812345678"}, "", ""},
		{"nothing reachable", Target{}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, recip := pickChannel(tt.target)
			if ch != tt.wantChan || recip != tt.wantRecip {
				t.Errorf("pickChannel = (%q,%q), want (%q,%q)", ch, recip, tt.wantChan, tt.wantRecip)
			}
		})
	}
}
