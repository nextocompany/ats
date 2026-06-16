package rbac

import "testing"

func TestKind(t *testing.T) {
	cases := map[string]string{
		"super_admin":        KindAll,
		"regional_director":  KindAll,
		"auditor":            KindAll,
		"operation_director": KindSubregion,
		"sgm":                KindStore,
		"hr_manager":         KindStore,
		"hr_staff":           KindStore,
		"something_unknown":  KindStore, // most-restrictive default
	}
	for role, want := range cases {
		if got := New(role, nil, "").Kind(); got != want {
			t.Errorf("role %q: got kind %q want %q", role, got, want)
		}
	}
}

func TestApplicationsClause(t *testing.T) {
	store := 7

	if c, args := New("super_admin", nil, "").ApplicationsClause(1); c != "" || args != nil {
		t.Errorf("admin should have no clause, got %q %v", c, args)
	}

	c, args := New("operation_director", nil, "Upper North").ApplicationsClause(3)
	if c == "" || len(args) != 1 || args[0] != "Upper North" {
		t.Errorf("subregion clause wrong: %q %v", c, args)
	}
	if want := "$3"; !contains(c, want) {
		t.Errorf("expected placeholder %s in %q", want, c)
	}

	c, args = New("hr_staff", &store, "").ApplicationsClause(1)
	if c == "" || len(args) != 1 || args[0] != store {
		t.Errorf("store clause wrong: %q %v", c, args)
	}

	if c, _ := New("hr_staff", nil, "").ApplicationsClause(1); c != "1=0" {
		t.Errorf("store-scoped user without store should match nothing, got %q", c)
	}
}

func TestCandidatesClause(t *testing.T) {
	store := 7

	// All-scope roles → no clause.
	if c, args := New("super_admin", nil, "").CandidatesClause(1); c != "" || args != nil {
		t.Errorf("admin should have no candidates clause, got %q %v", c, args)
	}

	// Subregion role → subregion = $N.
	c, args := New("operation_director", nil, "Upper North").CandidatesClause(2)
	if c == "" || len(args) != 1 || args[0] != "Upper North" || !contains(c, "$2") {
		t.Errorf("subregion candidates clause wrong: %q %v", c, args)
	}

	// Store role → subquery on applications.assigned_store_id.
	c, args = New("hr_manager", &store, "").CandidatesClause(1)
	if c == "" || len(args) != 1 || args[0] != store || !contains(c, "assigned_store_id") {
		t.Errorf("store candidates clause wrong: %q %v", c, args)
	}

	// Store role without a store → fail closed (matches nothing).
	if c, _ := New("hr_staff", nil, "").CandidatesClause(1); c != "1=0" {
		t.Errorf("store-scoped user without store should match no candidates, got %q", c)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
