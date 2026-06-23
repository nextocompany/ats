//go:build integration

package requisitions

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func dsn() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	return "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable"
}

func intp(v int) *int { return &v }

// mkPositionAndStore inserts a throwaway position + store (vacancies has FKs to
// both) and registers cleanup. Returns their ids.
func mkPositionAndStore(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, int) {
	t.Helper()
	ctx := context.Background()

	var posID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO positions (title_th, title_en, responsibilities, qualifications, benefits)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		"ตำแหน่งทดสอบ", "Test Position", "หน้าที่กลาง", "คุณสมบัติกลาง", "สวัสดิการกลาง").Scan(&posID); err != nil {
		t.Fatalf("insert position: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM positions WHERE id = $1`, posID) })

	// store_no is the PK; pick a high value unlikely to collide with seeded data.
	storeNo := 990041
	if _, err := pool.Exec(ctx,
		`INSERT INTO stores (store_no, store_name, subregion) VALUES ($1,$2,$3)
		 ON CONFLICT (store_no) DO NOTHING`,
		storeNo, "Test Store", "Upper North"); err != nil {
		t.Fatalf("insert store: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM stores WHERE store_no = $1`, storeNo) })

	return posID, storeNo
}

// TestCreate_RoundTripsDetailedFields proves the reqColumns projection, positional
// scanRequisition, and the Create INSERT all line up: a requisition created with
// every new JD/metadata field reads back identically (Create returns via getByID).
func TestCreate_RoundTripsDetailedFields(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated to 000040?): %v", err)
	}
	t.Cleanup(pool.Close)
	repo := NewRepository(pool)

	posID, storeNo := mkPositionAndStore(t, pool)

	in := CreateInput{
		PositionID:       posID,
		StoreID:          storeNo,
		Headcount:        3,
		Responsibilities: "ดูแลหน้าร้าน",
		Qualifications:   "วุฒิ ปวส. ขึ้นไป",
		Benefits:         "ประกันสุขภาพ + โบนัส",
		OtherDetails:     "ทำงานเป็นกะ",
		EmploymentType:   EmploymentContract,
		SalaryMin:        intp(15000),
		SalaryMax:        intp(20000),
		Priority:         PriorityUrgent,
		OpenReason:       ReasonReplacement,
	}
	got, err := repo.Create(ctx, in, uuid.New())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM vacancies WHERE id = $1`, got.ID) })

	if got.Status != StatusPendingApproval || got.Source != SourceManual {
		t.Fatalf("lifecycle: status=%q source=%q", got.Status, got.Source)
	}
	if got.Headcount != 3 {
		t.Fatalf("headcount: got %d", got.Headcount)
	}
	if got.Responsibilities != "ดูแลหน้าร้าน" || got.Qualifications != "วุฒิ ปวส. ขึ้นไป" {
		t.Fatalf("resp/qual mismatch: %+v", got)
	}
	if got.Benefits != "ประกันสุขภาพ + โบนัส" || got.OtherDetails != "ทำงานเป็นกะ" {
		t.Fatalf("benefits/other mismatch: %+v", got)
	}
	if got.EmploymentType != EmploymentContract || got.Priority != PriorityUrgent || got.OpenReason != ReasonReplacement {
		t.Fatalf("enum fields mismatch: emp=%q prio=%q reason=%q", got.EmploymentType, got.Priority, got.OpenReason)
	}
	if got.SalaryMin == nil || *got.SalaryMin != 15000 || got.SalaryMax == nil || *got.SalaryMax != 20000 {
		t.Fatalf("salary mismatch: min=%v max=%v", got.SalaryMin, got.SalaryMax)
	}
	// Joined label proves reqFrom still works.
	if got.PositionTitle == "" || got.StoreName != "Test Store" {
		t.Fatalf("joined labels: title=%q store=%q", got.PositionTitle, got.StoreName)
	}

	// Sparse Update round-trips and leaves untouched fields intact.
	newBenefits := "ปรับสวัสดิการใหม่"
	updated, err := repo.Update(ctx, got.ID, UpdateInput{Benefits: &newBenefits})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Benefits != newBenefits {
		t.Fatalf("update benefits: got %q", updated.Benefits)
	}
	if updated.Responsibilities != "ดูแลหน้าร้าน" || updated.Priority != PriorityUrgent {
		t.Fatalf("update clobbered untouched fields: %+v", updated)
	}

	// Nil salary round-trips as NULL -> nil.
	noSalary, err := repo.Create(ctx, CreateInput{PositionID: posID, StoreID: storeNo, Headcount: 1, Priority: PriorityNormal}, uuid.New())
	if err != nil {
		t.Fatalf("create no-salary: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM vacancies WHERE id = $1`, noSalary.ID) })
	if noSalary.SalaryMin != nil || noSalary.SalaryMax != nil {
		t.Fatalf("expected nil salary, got min=%v max=%v", noSalary.SalaryMin, noSalary.SalaryMax)
	}
}
