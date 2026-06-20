package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/nexto/hr-ats/pkg/config"
)

// Identity is the authenticated HR user extracted from an Entra token. It maps
// 1:1 onto middleware.DevUser; the middleware (not this package, to avoid an
// import cycle) does that conversion.
type Identity struct {
	ID        string
	Email     string
	Role      string
	StoreID   *int
	Subregion string
}

// EntraVerifier validates an Azure AD (Entra ID) bearer token and returns the
// HR identity. mockVerifier-style seam: only constructed when AUTH_PROVIDER=real.
type EntraVerifier interface {
	Verify(ctx context.Context, rawToken string) (Identity, error)
}

// TenantPolicy decides whether a directory (Entra tenant id) may sign in. The
// verifier cryptographically validates the token first; the policy is the
// authorisation gate layered on top. Implementations combine the static
// AZURE_AD_ALLOWED_TENANTS allowlist with the runtime "allow all tenants" admin
// toggle. tenantID is already lower-cased.
type TenantPolicy interface {
	AllowsTenant(ctx context.Context, tenantID string) bool
}

// entraClaims is the subset of the Entra token we consume. Role/store/subregion
// are app-specific claims configured on the Entra app registration. TenantID
// (`tid`) is enforced against the tenant policy.
type entraClaims struct {
	OID       string   `json:"oid"`
	TenantID  string   `json:"tid"`
	Email     string   `json:"preferred_username"`
	Roles     []string `json:"roles"`
	StoreID   *int     `json:"store_id"`
	Subregion string   `json:"subregion"`
}

// mapIdentity converts validated claims into an Identity. The first app role is
// used; an absent role yields "" which rbac treats as the most restrictive
// (store) scope — fail closed, never widen visibility.
func mapIdentity(c entraClaims) Identity {
	role := ""
	if len(c.Roles) > 0 {
		role = c.Roles[0]
	}
	return Identity{ID: c.OID, Email: c.Email, Role: role, StoreID: c.StoreID, Subregion: c.Subregion}
}

// oidcVerifier validates Entra ID tokens cryptographically, then defers the
// tenant authorisation decision to policy. Discovery always runs against the
// shared `organizations` endpoint so tokens from ANY directory can be validated;
// which tenants are actually accepted is the policy's call. This is what lets the
// admin "allow all tenants" toggle take effect at runtime without a restart.
type oidcVerifier struct {
	verifier *oidc.IDTokenVerifier
	policy   TenantPolicy
	clientID string // app (client) id; used to resolve directory-extension scope claims
}

// entraIssuer returns the v2.0 issuer URL for a tenant id.
func entraIssuer(tenant string) string {
	return fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", tenant)
}

// matchesTenantIssuer reports whether issuer is exactly tenant's v2.0 issuer.
// This issuer-binding stops a token claiming an allowed `tid` while being signed
// by a different directory.
func matchesTenantIssuer(issuer, tenant string) bool {
	return strings.EqualFold(issuer, entraIssuer(tenant))
}

// NewEntraVerifier performs OIDC discovery and returns a token verifier. It does
// network I/O, so it is constructed only when real auth is enabled (a discovery
// failure is a fatal startup error — fail fast).
//
// Discovery uses the shared `organizations` endpoint, whose metadata reports a
// templated issuer (".../{tenantid}/v2.0") that cannot match a fixed URL — so the
// expected issuer is overridden for discovery and the library issuer check is
// skipped. Verify then binds the concrete per-token issuer to its `tid` and asks
// the policy whether that tenant is allowed.
func NewEntraVerifier(ctx context.Context, cfg *config.Config, policy TenantPolicy) (EntraVerifier, error) {
	if policy == nil {
		return nil, fmt.Errorf("auth: nil tenant policy")
	}
	const orgIssuer = "https://login.microsoftonline.com/organizations/v2.0"
	discoveryCtx := oidc.InsecureIssuerURLContext(ctx, orgIssuer)
	provider, err := oidc.NewProvider(discoveryCtx, orgIssuer)
	if err != nil {
		return nil, fmt.Errorf("auth: entra discovery: %w", err)
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.AzureADClientID, SkipIssuerCheck: true})
	return oidcVerifier{verifier: verifier, policy: policy, clientID: cfg.AzureADClientID}, nil
}

func (v oidcVerifier) Verify(ctx context.Context, rawToken string) (Identity, error) {
	if rawToken == "" {
		return Identity{}, fmt.Errorf("auth: missing bearer token")
	}
	// Verifies signature (JWKS), aud, exp. Issuer is bound to the tenant below.
	tok, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return Identity{}, fmt.Errorf("auth: token verify: %w", err)
	}
	var claims entraClaims
	if err := tok.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("auth: claims: %w", err)
	}
	// Scope claims (store_id/subregion) may arrive short-named or as prefixed
	// directory-extension claims, so resolve them from the raw claim set rather
	// than the typed struct, which only matches the short name. A second decode
	// of the same token payload is safe (IDToken stores the raw claims bytes).
	var raw map[string]any
	if err := tok.Claims(&raw); err != nil {
		return Identity{}, fmt.Errorf("auth: raw claims: %w", err)
	}
	claims.StoreID = resolveStoreID(raw, v.clientID)
	claims.Subregion = resolveSubregion(raw, v.clientID)
	tid := strings.ToLower(strings.TrimSpace(claims.TenantID))
	if tid == "" {
		return Identity{}, fmt.Errorf("auth: token missing tenant id")
	}
	if !matchesTenantIssuer(tok.Issuer, tid) {
		return Identity{}, fmt.Errorf("auth: issuer %q does not match tenant %q", tok.Issuer, tid)
	}
	if !v.policy.AllowsTenant(ctx, tid) {
		return Identity{}, fmt.Errorf("auth: tenant %q not allowed", tid)
	}
	return mapIdentity(claims), nil
}
