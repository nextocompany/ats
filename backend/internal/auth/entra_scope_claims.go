package auth

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Store/subregion scope claims reach the token by one of two shapes:
//
//   - short name — `store_id` (number) / `subregion` (string). Only available via
//     a claims-mapping policy, which a multi-tenant app cannot emit without a
//     custom signing key.
//   - directory extension — `extension_<appid>_store_id` (string-typed, prefixed).
//     This is the supported path for this multi-tenant app: a directory extension
//     attribute set per user, surfaced as an optional claim.
//
// The resolvers below accept either shape and tolerate string- or number-typed
// values, so the Entra side can use whichever mechanism is configured without a
// code change. See docs/entra-app-roles-setup.md.

// extensionClaimPrefix builds the Entra directory-extension claim prefix for an
// app (client) id: "extension_" + the client id with dashes removed + "_".
func extensionClaimPrefix(clientID string) string {
	noDashes := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(clientID)), "-", "")
	return "extension_" + noDashes + "_"
}

// resolveStoreID returns the store_id claim as *int, preferring the short name
// and falling back to the directory-extension name. A blank or unparseable value
// yields nil — fail closed to "no store" rather than guess a wrong one.
func resolveStoreID(raw map[string]any, clientID string) *int {
	for _, key := range []string{"store_id", extensionClaimPrefix(clientID) + "store_id"} {
		if v, ok := raw[key]; ok {
			if n, ok := coerceInt(v); ok {
				return n
			}
		}
	}
	return nil
}

// resolveSubregion returns the subregion claim, preferring the short name and
// falling back to the directory-extension name. Absent or blank yields "".
func resolveSubregion(raw map[string]any, clientID string) string {
	for _, key := range []string{"subregion", extensionClaimPrefix(clientID) + "subregion"} {
		if v, ok := raw[key]; ok {
			if s, ok := v.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

// coerceInt converts a JSON-decoded claim value to *int. encoding/json decodes
// numbers into float64 by default, but directory-extension claims arrive as
// strings, so both are accepted. Returns false when the value cannot represent an
// integer.
func coerceInt(v any) (*int, bool) {
	switch n := v.(type) {
	case float64:
		i := int(n)
		return &i, true
	case json.Number:
		if i64, err := n.Int64(); err == nil {
			i := int(i64)
			return &i, true
		}
	case int:
		i := n
		return &i, true
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return nil, false
		}
		if i, err := strconv.Atoi(s); err == nil {
			return &i, true
		}
	}
	return nil, false
}
