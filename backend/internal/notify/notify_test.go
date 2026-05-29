package notify

import (
	"context"
	"testing"

	"github.com/nexto/hr-ats/pkg/config"
)

func TestNewNotifier_DefaultsToMock(t *testing.T) {
	n := NewNotifier(&config.Config{NotifyProvider: "mock"})
	if _, ok := n.(mockNotifier); !ok {
		t.Fatalf("expected mockNotifier by default, got %T", n)
	}
}

func TestNewNotifier_RealWhenConfigured(t *testing.T) {
	n := NewNotifier(&config.Config{NotifyProvider: "real", NotifyLINEToken: "tok"})
	if _, ok := n.(restNotifier); !ok {
		t.Fatalf("expected restNotifier when NOTIFY_PROVIDER=real, got %T", n)
	}
}

func TestMockNotifier_SendSucceeds(t *testing.T) {
	if err := (mockNotifier{}).Send(context.Background(), Message{
		Channel: ChannelLINE, Recipient: "U-123", Subject: "งานใหม่", Body: "มีตำแหน่งงานใหม่",
	}); err != nil {
		t.Fatalf("mock send returned error: %v", err)
	}
}
