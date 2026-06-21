package middleware

import (
	"io"
	"net"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func mustCIDRs(t *testing.T, list ...string) []*net.IPNet {
	t.Helper()
	nets, bad, masked := ParseTrustedCIDRs(list)
	if len(bad) != 0 || len(masked) != 0 {
		t.Fatalf("unexpected bad=%v masked=%v", bad, masked)
	}
	return nets
}

func TestParseTrustedCIDRs(t *testing.T) {
	nets, bad, masked := ParseTrustedCIDRs([]string{
		"10.0.0.0/8", " 100.64.0.1 ", "::1", "2001:db8::/32", "", "garbage", "999.1.1.1",
	})
	if len(nets) != 4 {
		t.Fatalf("expected 4 parsed nets, got %d (%v)", len(nets), nets)
	}
	if len(bad) != 2 { // "garbage", "999.1.1.1"
		t.Fatalf("expected 2 malformed entries, got %v", bad)
	}
	if len(masked) != 0 {
		t.Fatalf("expected no masked entries, got %v", masked)
	}
	// bare IPv4 became a /32 host route
	if !nets[1].Contains(net.ParseIP("100.64.0.1")) || nets[1].Contains(net.ParseIP("100.64.0.2")) {
		t.Errorf("bare IPv4 should parse as /32 host route, got %v", nets[1])
	}
	// bare IPv6 became a /128 host route
	if !nets[2].Contains(net.ParseIP("::1")) {
		t.Errorf("bare IPv6 should parse as /128, got %v", nets[2])
	}
}

func TestParseTrustedCIDRs_MaskedHostBits(t *testing.T) {
	nets, bad, masked := ParseTrustedCIDRs([]string{"10.224.5.3/24"})
	if len(bad) != 0 {
		t.Fatalf("expected no malformed entries, got %v", bad)
	}
	if len(nets) != 1 {
		t.Fatalf("expected the entry to still be applied, got %d nets", len(nets))
	}
	if len(masked) != 1 || masked[0] != "10.224.5.3/24 -> 10.224.5.0/24" {
		t.Fatalf("expected masked warning for widened CIDR, got %v", masked)
	}
}

func TestRealClientIP(t *testing.T) {
	trusted := mustCIDRs(t, "10.0.0.0/8", "172.16.0.0/12", "::1")

	tests := []struct {
		name      string
		forwarded []string
		remote    string
		trusted   []*net.IPNet
		want      string
	}{
		{
			name:    "no trusted proxies falls back to direct peer, ignoring XFF",
			trusted: nil,
			// even a present XFF must be ignored when we trust nobody
			forwarded: []string{"1.2.3.4"},
			remote:    "10.0.0.9",
			want:      "10.0.0.9",
		},
		{
			name:      "spoofed left-most entry is ignored; edge-appended client wins",
			forwarded: []string{"1.2.3.4", "203.0.113.9"},
			remote:    "10.0.0.5",
			trusted:   trusted,
			want:      "203.0.113.9",
		},
		{
			name:      "trailing trusted hops are skipped to reach the real client",
			forwarded: []string{"203.0.113.9", "10.0.0.7", "172.16.5.1"},
			remote:    "10.0.0.7",
			trusted:   trusted,
			want:      "203.0.113.9",
		},
		{
			name:      "attacker cannot prepend extra hops to bypass the limit",
			forwarded: []string{"66.66.66.66", "77.77.77.77", "203.0.113.9"},
			remote:    "10.0.0.5",
			trusted:   trusted,
			want:      "203.0.113.9", // right-most non-trusted, attacker entries unreachable
		},
		{
			name:      "empty XFF falls back to direct peer",
			forwarded: nil,
			remote:    "10.0.0.5",
			trusted:   trusted,
			want:      "10.0.0.5",
		},
		{
			name:      "all entries trusted falls back to direct peer",
			forwarded: []string{"10.1.1.1", "172.16.0.9"},
			remote:    "10.0.0.5",
			trusted:   trusted,
			want:      "10.0.0.5",
		},
		{
			name:      "invalid entries are skipped",
			forwarded: []string{"203.0.113.9", "not-an-ip", "10.0.0.7"},
			remote:    "10.0.0.7",
			trusted:   trusted,
			want:      "203.0.113.9",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := realClientIP(tc.forwarded, tc.remote, tc.trusted)
			if got != tc.want {
				t.Errorf("realClientIP(%v, %q) = %q, want %q", tc.forwarded, tc.remote, got, tc.want)
			}
		})
	}
}

// TestRealClientIP_ThroughFiber exercises the full c.IPs() → RealClientIP path via a
// real Fiber request, guarding against fiber changing X-Forwarded-For parsing.
func TestRealClientIP_ThroughFiber(t *testing.T) {
	trusted := mustCIDRs(t, "10.0.0.0/8")
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(RealClientIP(c, trusted))
	})

	tests := []struct {
		name string
		xff  string
		want string
	}{
		{"edge-appended client after spoofed prefix", "66.66.66.66, 203.0.113.9, 10.0.0.5", "203.0.113.9"},
		{"attacker cannot prepend hops", "1.1.1.1, 2.2.2.2, 203.0.113.9, 10.0.0.5", "203.0.113.9"},
		{"single real client", "203.0.113.9", "203.0.113.9"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Forwarded-For", tc.xff)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			body, _ := io.ReadAll(resp.Body)
			if string(body) != tc.want {
				t.Errorf("X-Forwarded-For %q → %q, want %q", tc.xff, string(body), tc.want)
			}
		})
	}
}

// TestResolveClientIPAndAuditActor exercises the audit-attribution path: the
// resolved IP is stashed in locals (ClientIP), and AuditActor reads the DevUser
// id + user agent + that IP. EnforceTrustProxies here trusts 10.0.0.0/8 so the
// edge-appended client wins.
func TestResolveClientIPAndAuditActor(t *testing.T) {
	trusted := mustCIDRs(t, "10.0.0.0/8")
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		// Simulate the auth middleware putting a DevUser in locals.
		c.Locals(UserContextKey, DevUser{ID: "11111111-1111-1111-1111-111111111111", Role: "super_admin"})
		return c.Next()
	})
	app.Use(ResolveClientIP(trusted))
	app.Get("/", func(c *fiber.Ctx) error {
		uid, ip, ua := AuditActor(c)
		got := "nil"
		if uid != nil {
			got = uid.String()
		}
		return c.SendString(got + "|" + ip + "|" + ua + "|local=" + ClientIP(c))
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.5")
	req.Header.Set("User-Agent", "test-agent/1.0")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	want := "11111111-1111-1111-1111-111111111111|203.0.113.9|test-agent/1.0|local=203.0.113.9"
	if string(body) != want {
		t.Errorf("AuditActor path: got %q want %q", string(body), want)
	}
}

// TestAuditActor_NonUUIDAndMissing covers a non-UUID DevUser id (→ nil) and the
// no-ResolveClientIP case (ClientIP → "").
func TestAuditActor_NonUUIDAndMissing(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(UserContextKey, DevUser{ID: "not-a-uuid", Role: "hr_staff"})
		return c.Next()
	})
	app.Get("/", func(c *fiber.Ctx) error {
		uid, ip, _ := AuditActor(c)
		if uid != nil {
			return c.SendString("UNEXPECTED-UID")
		}
		return c.SendString("nil-uid|ip=" + ip)
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "nil-uid|ip=" {
		t.Errorf("non-uuid/missing-resolve: got %q want %q", string(body), "nil-uid|ip=")
	}
}
