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

func TestAccountsClause(t *testing.T) {
	store := 7

	// All-scope roles → no clause (every account, incl. 0-application ones).
	if c, args := New("super_admin", nil, "").AccountsClause("a", 1); c != "" || args != nil {
		t.Errorf("admin should have no accounts clause, got %q %v", c, args)
	}

	// Subregion role → correlated EXISTS on candidates.subregion, using the alias.
	c, args := New("operation_director", nil, "Upper North").AccountsClause("a", 2)
	if c == "" || len(args) != 1 || args[0] != "Upper North" || !contains(c, "$2") ||
		!contains(c, "c.account_id = a.id") || !contains(c, "c.subregion") {
		t.Errorf("subregion accounts clause wrong: %q %v", c, args)
	}

	// Store role → correlated EXISTS joining applications.assigned_store_id.
	c, args = New("hr_manager", &store, "").AccountsClause("a", 1)
	if c == "" || len(args) != 1 || args[0] != store ||
		!contains(c, "assigned_store_id") || !contains(c, "c.account_id = a.id") {
		t.Errorf("store accounts clause wrong: %q %v", c, args)
	}

	// Store role without a store → fail closed (matches nothing).
	if c, _ := New("hr_staff", nil, "").AccountsClause("a", 1); c != "1=0" {
		t.Errorf("store-scoped user without store should match no accounts, got %q", c)
	}

	// AllScope bypasses role: even a store role string yields no clause.
	if New("hr_staff", &store, "").all != false {
		t.Fatal("New should not set the all bypass")
	}
	if k := AllScope().Kind(); k != KindAll {
		t.Errorf("AllScope().Kind() = %q, want %q", k, KindAll)
	}
	if c, args := AllScope().AccountsClause("a", 1); c != "" || args != nil {
		t.Errorf("AllScope should produce no accounts clause, got %q %v", c, args)
	}
}

func TestAreaAndRequisitionKinds(t *testing.T) {
	if got := New("area_hr", nil, "").Kind(); got != KindArea {
		t.Errorf("area_hr kind = %q, want %q", got, KindArea)
	}
	for _, role := range []string{"hiring_manager_store", "hiring_manager_ho"} {
		if got := New(role, nil, "").Kind(); got != KindRequisition {
			t.Errorf("%s kind = %q, want %q", role, got, KindRequisition)
		}
	}
	if got := New("ta", nil, "").Kind(); got != KindAll {
		t.Errorf("ta kind = %q, want %q", got, KindAll)
	}
	if got := New("hr_store", nil, "").Kind(); got != KindStore {
		t.Errorf("hr_store kind = %q, want %q", got, KindStore)
	}
}

func TestAreaClause(t *testing.T) {
	uid := "11111111-1111-1111-1111-111111111111"

	// Area scope resolves stores via user_areas/area_stores and includes the pool.
	c, args := New("area_hr", nil, "").WithUserID(uid).ApplicationsClause(1)
	if len(args) != 1 || args[0] != uid {
		t.Fatalf("area clause args wrong: %q %v", c, args)
	}
	for _, want := range []string{"area_stores", "user_areas", "$1::uuid", "talent_pool"} {
		if !contains(c, want) {
			t.Errorf("area applications clause missing %q: %q", want, c)
		}
	}

	// Area scope without a user id fails closed.
	if c, _ := New("area_hr", nil, "").ApplicationsClause(1); c != "1=0" {
		t.Errorf("area scope without user id should match nothing, got %q", c)
	}
}

func TestRequisitionClause(t *testing.T) {
	uid := "22222222-2222-2222-2222-222222222222"

	// Hiring manager sees only candidates in their own requisitions, NO pool.
	c, args := New("hiring_manager_ho", nil, "").WithUserID(uid).ApplicationsClause(2)
	if len(args) != 1 || args[0] != uid || !contains(c, "vacancy_id") || !contains(c, "hiring_manager_user_id") || !contains(c, "$2::uuid") {
		t.Errorf("requisition applications clause wrong: %q %v", c, args)
	}
	if contains(c, "talent_pool") {
		t.Errorf("requisition scope must NOT include the central pool: %q", c)
	}

	// Vacancies clause for a hiring manager is their owned requisitions.
	if c, _ := New("hiring_manager_store", nil, "").WithUserID(uid).VacanciesClause(1); !contains(c, "hiring_manager_user_id = $1::uuid") {
		t.Errorf("requisition vacancies clause wrong: %q", c)
	}

	// No user id → fail closed.
	if c, _ := New("hiring_manager_store", nil, "").ApplicationsClause(1); c != "1=0" {
		t.Errorf("requisition scope without user id should match nothing, got %q", c)
	}
}

func TestStoreScopeIncludesPool(t *testing.T) {
	store := 7
	c, _ := New("hr_store", &store, "").ApplicationsClause(1)
	if !contains(c, "assigned_store_id = $1") || !contains(c, "talent_pool = TRUE AND assigned_store_id IS NULL") {
		t.Errorf("store scope should include own store + central pool: %q", c)
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
