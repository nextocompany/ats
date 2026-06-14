package settings

import (
	"context"
	"sync"
	"time"
)

// KeyAllowAllTenants is the flag that, when true, lets a token from ANY Entra
// directory sign in (subject to the verifier's issuer-binding). When false only
// the static AZURE_AD_ALLOWED_TENANTS allowlist is honoured.
const KeyAllowAllTenants = "allow_all_entra_tenants"

// cacheTTL bounds how stale the auth hot-path read may be. The toggle is read on
// every HR request, so we cache it briefly instead of hitting Postgres each time;
// a flip takes effect within this window without a restart.
const cacheTTL = 10 * time.Second

// Service reads/writes system settings with a short read-through cache for the
// auth hot path. Writes go straight to the repository and bust the cache.
type Service struct {
	repo Repository
	now  func() time.Time

	mu    sync.Mutex
	cache map[string]cachedBool
}

type cachedBool struct {
	val  bool
	at   time.Time
	seen bool
}

// NewService builds a settings service over the repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now, cache: make(map[string]cachedBool)}
}

// AllowAllTenants reports the cached "allow all Entra tenants" flag. It is the
// auth hot-path reader: on a cache miss it queries the repository; on a query
// error it returns the last known value, defaulting to false (fail closed) so a
// transient DB blip never silently opens sign-in to every tenant.
func (s *Service) AllowAllTenants(ctx context.Context) bool {
	return s.getBoolCached(ctx, KeyAllowAllTenants)
}

func (s *Service) getBoolCached(ctx context.Context, key string) bool {
	now := s.now()

	s.mu.Lock()
	entry, ok := s.cache[key]
	if ok && now.Sub(entry.at) < cacheTTL {
		s.mu.Unlock()
		return entry.val
	}
	s.mu.Unlock()

	val, err := s.repo.GetBool(ctx, key)
	if err != nil {
		// Fail closed: keep the last good value if we have one, else false.
		if ok {
			return entry.val
		}
		return false
	}

	s.mu.Lock()
	s.cache[key] = cachedBool{val: val, at: now, seen: true}
	s.mu.Unlock()
	return val
}

// GetAllowAll reads the flag straight from the store (no cache) for the admin
// read endpoint, so the console always shows the persisted truth.
func (s *Service) GetAllowAll(ctx context.Context) (bool, error) {
	return s.repo.GetBool(ctx, KeyAllowAllTenants)
}

// SetAllowAll persists the flag, records the admin who changed it, and busts the
// cache so the new value is read immediately on the next request.
func (s *Service) SetAllowAll(ctx context.Context, val bool, updatedBy string) error {
	if err := s.repo.SetBool(ctx, KeyAllowAllTenants, val, updatedBy); err != nil {
		return err
	}
	s.mu.Lock()
	s.cache[KeyAllowAllTenants] = cachedBool{val: val, at: s.now(), seen: true}
	s.mu.Unlock()
	return nil
}
