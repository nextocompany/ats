package rbac

import (
	"context"
	"sort"
	"testing"
)

// stubReader feeds the authorizer a fixed role set without a DB.
type stubReader struct{ roles []Role }

func (s stubReader) ListRoles(_ context.Context) ([]Role, error) { return s.roles, nil }

// builtinScope / seedMatrix reference the single canonical legacy source in
// legacy.go (which mirrors migration 000028's seed). The parity test below
// cross-checks them against the INDEPENDENTLY-transcribed oldAllowlists, so a
// transcription error in either legacy.go or the migration is caught.
var builtinScope = legacyRoleScope
var seedMatrix = legacyRolePerms

// oldAllowlists is the legacy compile-time matrix, transcribed permission-by-
// permission from the Go allowlists this feature replaces (super_admin excluded;
// rbac.admin is net-new so it has no legacy entry). If this and the seed diverge,
// the cutover would silently change access — so this test must stay green.
var oldAllowlists = map[string][]string{
	PermSettingsAdmin:    {},                                                                                      // settings.adminRolesAllowed
	PermUsersAdmin:       {},                                                                                      // hrauth requireSuperAdmin
	PermExecutiveView:    {"regional_director", "auditor"},                                                        // executiveRolesAllowed
	PermReportsView:      {"regional_director", "auditor", "operation_director", "sgm", "hr_manager", "hr_staff"}, // reportViewRoles (all7)
	PermReportsExport:    {"regional_director"},                                                                   // exportRolesAllowed
	PermReengageTrigger:  {"regional_director", "operation_director"},                                             // reengage rolesAllowed
	PermMembersAdmin:     {"hr_manager"},                                                                          // memberAdminRoles
	PermMembersErase:     {},                                                                                      // memberEraseRoles
	PermBulkUpload:       {"hr_manager", "sgm", "hr_staff"},                                                       // bulkIntakeRoles
	PermAssignmentWrite:  {"hr_manager", "sgm"},                                                                   // assignmentRoles
	PermOfferWrite:       {"hr_manager"},                                                                          // offerWriteRoles
	PermOnboardingWrite:  {"hr_manager", "hr_staff", "sgm"},                                                       // onboardingWriteRoles
	PermLetterWrite:      {"hr_manager", "hr_staff", "sgm"},                                                       // letterWriteRoles
	PermScorecardTA:      {"hr_manager", "hr_staff"},                                                              // taRecordRoles
	PermScorecardLM:      {"sgm"},                                                                                 // lmRecordRoles
	PermApprovalSubmit:   {"hr_staff"},                                                                            // canSubmitApproval
	PermApprovalDecideL1: {"hr_staff"},                                                                            // approvalLevelRoles[1]
	PermApprovalDecideL2: {"hr_manager"},                                                                          // approvalLevelRoles[2]
	PermApprovalDecideL3: {"sgm"},                                                                                 // approvalLevelRoles[3]
	PermApprovalDecideL4: {"regional_director"},                                                                   // approvalLevelRoles[4]
}

var builtinRoleKeys = []string{
	"super_admin", "regional_director", "auditor", "operation_director", "sgm", "hr_manager", "hr_staff",
}

func newSeededAuthorizer(t *testing.T) *Authorizer {
	t.Helper()
	roles := make([]Role, 0, len(builtinRoleKeys))
	for _, key := range builtinRoleKeys {
		perms := seedMatrix[key] // nil for super_admin → bypass covers it
		roles = append(roles, Role{Key: key, ScopeKind: builtinScope[key], IsBuiltin: true, Permissions: perms})
	}
	a := NewAuthorizer(stubReader{roles: roles}, 0)
	if err := a.Reload(context.Background()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	return a
}

// TestSeedMatrixParity is the safety net: the seeded authorizer must grant exactly
// what the legacy allowlists granted, for every (role, permission) pair.
func TestSeedMatrixParity(t *testing.T) {
	a := newSeededAuthorizer(t)
	for _, perm := range AllPermissions {
		if perm == PermRBACAdmin || perm == PermRequisitionManage || perm == PermRequisitionApprove || perm == PermBreachManage || perm == PermPDPAAdmin {
			continue // net-new permissions, no legacy allowlist equivalent
		}
		allowed, ok := oldAllowlists[perm]
		if !ok {
			t.Fatalf("permission %q has no legacy allowlist entry in the test — add it", perm)
		}
		allow := map[string]bool{}
		for _, r := range allowed {
			allow[r] = true
		}
		for _, role := range builtinRoleKeys {
			want := role == "super_admin" || allow[role] // super_admin was in every old allowlist
			if got := a.Can(role, perm); got != want {
				t.Errorf("Can(%q, %q) = %v, want %v (legacy parity)", role, perm, got, want)
			}
		}
	}
}

// TestLegacyScopeParity asserts the seeded scope kinds equal the old hardcoded
// switch in scope.go (super_admin/regional_director/auditor=all,
// operation_director=subregion, everything else=store).
func TestLegacyScopeParity(t *testing.T) {
	a := newSeededAuthorizer(t)
	legacy := func(role string) string {
		switch role {
		case "super_admin", "regional_director", "auditor":
			return KindAll
		case "operation_director":
			return KindSubregion
		default:
			return KindStore
		}
	}
	for _, role := range builtinRoleKeys {
		if got, want := a.ScopeKind(role), legacy(role); got != want {
			t.Errorf("ScopeKind(%q) = %q, want %q", role, got, want)
		}
	}
}

func TestSuperAdminBypass(t *testing.T) {
	a := newSeededAuthorizer(t)
	for _, perm := range AllPermissions {
		if !a.Can("super_admin", perm) {
			t.Errorf("super_admin should hold %q", perm)
		}
	}
	if a.ScopeKind("super_admin") != KindAll {
		t.Errorf("super_admin scope should be %q", KindAll)
	}
	if len(a.Permissions("super_admin")) != len(AllPermissions) {
		t.Errorf("super_admin should list all %d permissions", len(AllPermissions))
	}
}

func TestUnknownRoleFailsClosed(t *testing.T) {
	a := newSeededAuthorizer(t)
	if a.Can("ghost", PermReportsView) {
		t.Error("unknown role must not be granted any permission")
	}
	if a.ScopeKind("ghost") != KindStore {
		t.Errorf("unknown role scope must fail closed to %q", KindStore)
	}
	if len(a.Permissions("ghost")) != 0 {
		t.Error("unknown role must have no permissions")
	}
	if a.Can("", PermReportsView) || a.ScopeKind("") != KindStore {
		t.Error("empty role must fail closed")
	}
}

func TestPermissionsReflectMatrix(t *testing.T) {
	a := newSeededAuthorizer(t)
	got := a.Permissions("hr_staff")
	sort.Strings(got)
	want := append([]string(nil), seedMatrix["hr_staff"]...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("hr_staff permissions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("hr_staff permissions = %v, want %v", got, want)
		}
	}
}

// TestReloadPicksUpChanges proves a matrix edit is reflected after Reload (the
// per-replica propagation mechanism).
func TestReloadPicksUpChanges(t *testing.T) {
	reader := &stubReader{roles: []Role{{Key: "custom", ScopeKind: KindStore, Permissions: []string{PermReportsView}}}}
	a := NewAuthorizer(reader, 0)
	if err := a.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a.Can("custom", PermOfferWrite) {
		t.Fatal("custom should not have offer.write yet")
	}
	reader.roles = []Role{{Key: "custom", ScopeKind: KindStore, Permissions: []string{PermReportsView, PermOfferWrite}}}
	if err := a.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !a.Can("custom", PermOfferWrite) {
		t.Fatal("custom should have offer.write after reload")
	}
}
