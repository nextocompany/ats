package middleware

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// clientIPLocalKey is the Locals key under which ResolveClientIP stashes the
// spoof-resistant client IP for the request.
const clientIPLocalKey = "audit_client_ip"

// ResolveClientIP stashes the spoof-resistant client IP (RealClientIP) into the
// request Locals so any downstream handler can read it via ClientIP without
// re-threading the trusted-proxy list. Install once, after the trust config is
// parsed and before the route groups.
func ResolveClientIP(trusted []*net.IPNet) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals(clientIPLocalKey, RealClientIP(c, trusted))
		return c.Next()
	}
}

// ClientIP returns the resolved client IP stashed by ResolveClientIP, or "" when
// the middleware is not installed.
func ClientIP(c *fiber.Ctx) string {
	if v, ok := c.Locals(clientIPLocalKey).(string); ok {
		return v
	}
	return ""
}

// AuditActor extracts the audit attribution for the current request: the
// authenticated HR user's id (parsed from the DevUser/JWT claim, nil if absent or
// not a UUID), the spoof-resistant client IP, and the user agent. Candidate
// self-service handlers (no DevUser) pass their own account id instead.
func AuditActor(c *fiber.Ctx) (userID *uuid.UUID, ip, userAgent string) {
	ip = ClientIP(c)
	userAgent = c.Get(fiber.HeaderUserAgent)
	if u, ok := c.Locals(UserContextKey).(DevUser); ok && u.ID != "" {
		if id, err := uuid.Parse(u.ID); err == nil {
			userID = &id
		}
	}
	return userID, ip, userAgent
}

// ParseTrustedCIDRs turns the TRUSTED_PROXIES allowlist (IPs or CIDRs) into parsed
// networks. A bare IP becomes a host route (/32 or /128). Malformed entries are
// skipped and returned in bad. Entries whose host bits were set (e.g. "10.0.0.5/24",
// which net.ParseCIDR silently widens to 10.0.0.0/24 — trusting 255 extra hosts) are
// still applied but returned in masked so the caller can warn: a copy-paste mistake
// here silently widens the trust boundary.
func ParseTrustedCIDRs(list []string) (nets []*net.IPNet, bad, masked []string) {
	for _, raw := range list {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if !strings.Contains(s, "/") {
			if strings.Contains(s, ":") {
				s += "/128"
			} else {
				s += "/32"
			}
		}
		host, n, err := net.ParseCIDR(s)
		if err != nil {
			bad = append(bad, raw)
			continue
		}
		if !host.Equal(n.IP) {
			masked = append(masked, strings.TrimSpace(raw)+" -> "+n.String())
		}
		nets = append(nets, n)
	}
	return nets, bad, masked
}

// RealClientIP resolves the rate-limiting key from a Fiber request in a way that
// resists X-Forwarded-For spoofing. See realClientIP for the algorithm.
func RealClientIP(c *fiber.Ctx, trusted []*net.IPNet) string {
	return realClientIP(c.IPs(), c.Context().RemoteIP().String(), trusted)
}

// realClientIP returns the right-most X-Forwarded-For entry that is NOT a trusted
// proxy — i.e. the address our own trusted edge (the ACA ingress) actually observed.
// Because a trusted proxy only ever APPENDS to X-Forwarded-For, any value a client
// injects sits to the LEFT of the edge-appended real address and is never reached,
// so the key cannot be spoofed. When no proxy is trusted (dev/CI) or the header is
// absent/fully trusted, we fall back to the direct TCP peer — never a client-
// supplied value.
func realClientIP(forwarded []string, remoteIP string, trusted []*net.IPNet) string {
	if len(trusted) == 0 {
		return remoteIP
	}
	for i := len(forwarded) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(forwarded[i]))
		if ip == nil {
			continue
		}
		if !ipInAny(ip, trusted) {
			return ip.String()
		}
	}
	return remoteIP
}

func ipInAny(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientIPDebugLogger logs the raw X-Forwarded-For chain, the direct peer, and the
// resolved client IP for each request — ONLY when enabled. Used once on prod to
// confirm how the ACA ingress populates X-Forwarded-For before trusting it for rate
// limiting; default-off so it is safe to leave wired in.
func ClientIPDebugLogger(enabled bool, trusted []*net.IPNet) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if enabled {
			log.Info().
				Strs("xff", c.IPs()).
				Str("remote_ip", c.Context().RemoteIP().String()).
				Str("resolved", RealClientIP(c, trusted)).
				Str("path", c.Path()).
				Msg("client-ip debug")
		}
		return c.Next()
	}
}
