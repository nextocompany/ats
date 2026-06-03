// Command importref loads real reference data (stores + Master JD positions)
// from CSV into PostgreSQL. Idempotent upserts let it run repeatedly and lets
// real Storelist.csv / Master JD files replace the synthetic seed.
//
// Usage: importref <stores.csv> <positions.csv>
package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/stores"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: importref <stores.csv> <positions.csv>")
		os.Exit(2)
	}
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		dsn = "postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable" //#nosec G101 -- local-dev fallback DSN only (mirrors docker-compose/Makefile); real deployments set DB_URL.
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fatal("connect db", err)
	}
	defer pool.Close()

	ns, err := importStores(ctx, pool, os.Args[1])
	if err != nil {
		fatal("import stores", err)
	}
	np, err := importPositions(ctx, pool, os.Args[2])
	if err != nil {
		fatal("import positions", err)
	}
	fmt.Printf("imported %d stores, %d positions\n", ns, np)
}

func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, "importref: %s: %v\n", msg, err)
	os.Exit(1)
}

// readCSV returns header-indexed rows.
func readCSV(path string) ([]map[string]string, error) {
	f, err := os.Open(path) //#nosec G304,G703 -- importref is an operator-run admin CLI; path is an explicit argv CSV path, not untrusted external input.
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 1 {
		return nil, nil
	}
	header := records[0]
	var out []map[string]string
	for _, rec := range records[1:] {
		row := make(map[string]string, len(header))
		for i, h := range header {
			if i < len(rec) {
				row[strings.TrimSpace(h)] = strings.TrimSpace(rec[i])
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func importStores(ctx context.Context, pool *pgxpool.Pool, path string) (int, error) {
	rows, err := readCSV(path)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, row := range rows {
		storeNo, err := strconv.Atoi(row["store_no"])
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip store (bad store_no %q)\n", row["store_no"])
			continue
		}
		province := row["province"]
		subregion := row["subregion"]
		if subregion == "" {
			subregion = stores.ResolveSubregion(province)
		}
		lat := parseFloatPtr(row["latitude"])
		lng := parseFloatPtr(row["longitude"])
		if lat == nil || lng == nil {
			if la, lo, ok := stores.ProvinceCentroid(province); ok {
				lat, lng = &la, &lo
			}
		}

		const q = `
			INSERT INTO stores (store_no, store_name, format_type, subregion, province, latitude, longitude)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (store_no) DO UPDATE SET
				store_name=EXCLUDED.store_name, format_type=EXCLUDED.format_type,
				subregion=EXCLUDED.subregion, province=EXCLUDED.province,
				latitude=EXCLUDED.latitude, longitude=EXCLUDED.longitude`
		if _, err := pool.Exec(ctx, q, storeNo, row["store_name"], row["format_type"], subregion, province, lat, lng); err != nil {
			return n, fmt.Errorf("upsert store %d: %w", storeNo, err)
		}
		n++
	}
	return n, nil
}

func importPositions(ctx context.Context, pool *pgxpool.Pool, path string) (int, error) {
	rows, err := readCSV(path)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, row := range rows {
		code := row["ps_position_code"]
		if code == "" {
			fmt.Fprintf(os.Stderr, "skip position %q (missing ps_position_code, not idempotent)\n", row["title_en"])
			continue
		}
		mustHave := fmt.Sprintf(`{"min_education_level":%d,"min_experience_months":%d}`,
			atoiDefault(row["min_education_level"], 0), atoiDefault(row["min_experience_months"], 0))
		var keywords []string
		if row["keywords"] != "" {
			keywords = strings.Split(row["keywords"], "|")
		}

		const q = `
			INSERT INTO positions (title_th, title_en, level, ps_position_code, must_have_criteria, keywords, is_active)
			VALUES ($1,$2,$3,$4,$5::jsonb,$6,TRUE)
			ON CONFLICT (ps_position_code) WHERE ps_position_code IS NOT NULL DO UPDATE SET
				title_th=EXCLUDED.title_th, title_en=EXCLUDED.title_en, level=EXCLUDED.level,
				must_have_criteria=EXCLUDED.must_have_criteria, keywords=EXCLUDED.keywords, is_active=TRUE`
		if _, err := pool.Exec(ctx, q, row["title_th"], row["title_en"], row["level"], code, mustHave, keywords); err != nil {
			return n, fmt.Errorf("upsert position %q: %w", code, err)
		}
		n++
	}
	return n, nil
}

func parseFloatPtr(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func atoiDefault(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}
