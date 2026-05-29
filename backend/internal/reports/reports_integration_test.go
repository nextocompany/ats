//go:build integration

package reports

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

func TestFunnelAndSources(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connect (stack up + migrated?): %v", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE applications, candidates, positions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	var pos, cWeb, cWalk uuid.UUID
	pool.QueryRow(ctx, `INSERT INTO positions (title_th) VALUES ('t') RETURNING id`).Scan(&pos)
	pool.QueryRow(ctx, `INSERT INTO candidates (full_name, source_channel, status) VALUES ('w','career_portal','available') RETURNING id`).Scan(&cWeb)
	pool.QueryRow(ctx, `INSERT INTO candidates (full_name, source_channel, status) VALUES ('k','walk_in','available') RETURNING id`).Scan(&cWalk)

	// 2 career_portal (1 hired, must_have true), 1 walk_in (rejected)
	pool.Exec(ctx, `INSERT INTO applications (candidate_id, position_id, status, must_have_passed) VALUES
		($1,$3,'hired',true),($1,$3,'scored',true),($2,$3,'rejected',false)`, cWeb, cWalk, pos)

	r := New(pool)

	f, err := r.Funnel(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if f.Applied != 3 || f.PassedAI != 2 || f.Hired != 1 {
		t.Errorf("funnel wrong: %+v", f)
	}

	sources, err := r.Sources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var web Source
	for _, s := range sources {
		if s.Channel == "career_portal" {
			web = s
		}
	}
	if web.Applied != 2 || web.Hired != 1 || web.Conversion != 0.5 {
		t.Errorf("career_portal source wrong: %+v", web)
	}
}
