package auth

import (
	"context"
	"fmt"

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

// entraClaims is the subset of the Entra token we consume. Role/store/subregion
// are app-specific claims configured on the Entra app registration.
type entraClaims struct {
	OID       string   `json:"oid"`
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

type oidcVerifier struct{ verifier *oidc.IDTokenVerifier }

// NewEntraVerifier performs OIDC discovery against the tenant and returns a
// token verifier. It does network I/O, so it is constructed only when real auth
// is enabled (a discovery failure is a fatal startup error — fail fast).
func NewEntraVerifier(ctx context.Context, cfg *config.Config) (EntraVerifier, error) {
	issuer := fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", cfg.AzureADTenantID)
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: entra discovery: %w", err)
	}
	return oidcVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: cfg.AzureADClientID})}, nil
}

func (v oidcVerifier) Verify(ctx context.Context, rawToken string) (Identity, error) {
	if rawToken == "" {
		return Identity{}, fmt.Errorf("auth: missing bearer token")
	}
	tok, err := v.verifier.Verify(ctx, rawToken) // checks signature (JWKS), iss, aud, exp
	if err != nil {
		return Identity{}, fmt.Errorf("auth: token verify: %w", err)
	}
	var claims entraClaims
	if err := tok.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("auth: claims: %w", err)
	}
	return mapIdentity(claims), nil
}
