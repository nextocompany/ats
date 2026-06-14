package auth

import "testing"

func intPtr(n int) *int { return &n }

func TestMatchesTenantIssuer(t *testing.T) {
	const tid = "11111111-1111-1111-1111-111111111111"
	cases := []struct {
		name   string
		issuer string
		tenant string
		want   bool
	}{
		{"exact match", entraIssuer(tid), tid, true},
		{"case-insensitive", "HTTPS://LOGIN.MICROSOFTONLINE.COM/" + tid + "/V2.0", tid, true},
		{"issuer for a different tenant", entraIssuer("99999999-9999-9999-9999-999999999999"), tid, false},
		{"non-microsoft issuer", "https://evil.example.com/" + tid + "/v2.0", tid, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchesTenantIssuer(tc.issuer, tc.tenant); got != tc.want {
				t.Fatalf("matchesTenantIssuer(%q,%q)=%v want %v", tc.issuer, tc.tenant, got, tc.want)
			}
		})
	}
}

func TestMapIdentity(t *testing.T) {
	cases := []struct {
		name   string
		claims entraClaims
		want   Identity
	}{
		{
			name:   "full claims, first role wins",
			claims: entraClaims{OID: "o1", Email: "a@x.com", Roles: []string{"regional_director", "auditor"}, StoreID: intPtr(7), Subregion: "East"},
			want:   Identity{ID: "o1", Email: "a@x.com", Role: "regional_director", StoreID: intPtr(7), Subregion: "East"},
		},
		{
			name:   "no roles → empty (fail closed)",
			claims: entraClaims{OID: "o2", Email: "b@x.com"},
			want:   Identity{ID: "o2", Email: "b@x.com", Role: ""},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapIdentity(tc.claims)
			if got.ID != tc.want.ID || got.Email != tc.want.Email || got.Role != tc.want.Role || got.Subregion != tc.want.Subregion {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
			if (got.StoreID == nil) != (tc.want.StoreID == nil) {
				t.Fatalf("storeID nil-ness mismatch: got %v want %v", got.StoreID, tc.want.StoreID)
			}
			if got.StoreID != nil && *got.StoreID != *tc.want.StoreID {
				t.Fatalf("storeID got %d want %d", *got.StoreID, *tc.want.StoreID)
			}
		})
	}
}
