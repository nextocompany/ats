package executive

import "context"

// roi_mock.go produces a deterministic Recruitment ROI & Performance payload for
// the demo. Synthetic hires/funnel/success are derived from stable seeds over the
// real (or baked) store/position names — no math/rand, no time.Now — so refreshes
// and tests are repeatable. The cost-driven figures still read the real admin
// CostConfig (via getCostConfig) and run the SAME applyROIMath as live, so the
// cost editor drives the mock dashboard too and the ROI empty-state shows when
// assumptions are unset.

// ROI returns the deterministic mock ROIView.
func (m *mockService) ROI(ctx context.Context, f ExecFilters) (ROIView, error) {
	f.Dimension = normalizeDimension(f.Dimension)
	months := monthsInPeriod(f.Period)

	cost, err := getCostConfig(ctx, m.pool)
	if err != nil {
		return ROIView{}, err
	}

	stores := m.loadStores(ctx)
	positions := m.loadPositions(ctx)

	// Deterministic funnel volumes scaled by the period length.
	applied := (820 + seedSpan(months, 400)) * months
	screened := applied * 46 / 100
	interviewed := screened * 38 / 100
	offered := interviewed * 55 / 100
	hired := offered * 72 / 100
	responded := applied * 84 / 100
	avgDays := 12.5 + float64(seedSpan(months*3, 70))/10.0 // ~12–19 days

	funnel := FunnelStat{
		Applied:          applied,
		Screened:         screened,
		Interviewed:      interviewed,
		Offered:          offered,
		Hired:            hired,
		ResponseRate:     pct(responded, applied),
		ConversionToHire: pct(hired, applied),
	}

	success := mockSuccess(f.Dimension, stores, positions, months)

	view := ROIView{
		DataSource: "mock",
		Period:     f.Period,
		Dimension:  f.Dimension,
		Cost:       cost,
		Hires:      hired,
		Funnel:     funnel,
		TimeToHire: TimeToHire{
			Hires:      hired,
			AvgDays:    round1(avgDays),
			MedianDays: round1(avgDays - 1.5),
		},
		Success: success,
	}
	applyROIMath(&view, cost, hired, avgDays, months)
	return view, nil
}

// mockSuccess synthesizes per-dimension rows over real dimension names so Σ rows
// reconciles to the headline hired count.
func mockSuccess(dimension string, stores []storeRef, positions []PipelinePosition, months int) []SuccessRow {
	sources := []string{"LINE", "Google", "Walk-in", "Referral", "JobsDB"}
	rows := make([]SuccessRow, 0)

	switch dimension {
	case "position":
		for i, p := range positions {
			apps := (120 + i*40) * months
			hires := apps * (10 + seedSpan(i+1, 12)) / 100
			rows = append(rows, SuccessRow{
				Key:           p.PositionID,
				Label:         p.Title,
				Applications:  apps,
				Hires:         hires,
				Conversion:    pct(hires, apps),
				AvgTimeToHire: round1(13 + float64(seedSpan(i*5, 80))/10.0),
				TopSource:     sources[i%len(sources)],
			})
		}
	case "region":
		seen := map[string]int{}
		idx := 0
		for _, s := range stores {
			region := s.subregion
			if region == "" {
				region = "Unmapped"
			}
			pos, ok := seen[region]
			if !ok {
				pos = len(rows)
				seen[region] = pos
				rows = append(rows, SuccessRow{Key: "region:" + region, Label: region, TopSource: sources[idx%len(sources)]})
				idx++
			}
			apps := (90 + seedSpan(s.no, 120)) * months
			hires := apps * (8 + seedSpan(s.no*3, 14)) / 100
			rows[pos].Applications += apps
			rows[pos].Hires += hires
		}
		for i := range rows {
			rows[i].Conversion = pct(rows[i].Hires, rows[i].Applications)
			rows[i].AvgTimeToHire = round1(14 + float64(seedSpan(i*7, 60))/10.0)
		}
	default: // branch
		for i, s := range stores {
			apps := (90 + seedSpan(s.no, 150)) * months
			hires := apps * (8 + seedSpan(s.no*3, 15)) / 100
			rows = append(rows, SuccessRow{
				Key:           intToStr(s.no),
				Label:         s.name,
				Applications:  apps,
				Hires:         hires,
				Conversion:    pct(hires, apps),
				AvgTimeToHire: round1(13 + float64(seedSpan(s.no*4, 70))/10.0),
				TopSource:     sources[i%len(sources)],
			})
		}
	}
	return rows
}

// GetCostConfig / SetCostConfig satisfy the Service cost-config seam (mock).
// They are DB-backed so the admin editor behaves identically under either provider.
func (m *mockService) GetCostConfig(ctx context.Context) (CostConfig, error) {
	return getCostConfig(ctx, m.pool)
}

func (m *mockService) SetCostConfig(ctx context.Context, c CostConfig, updatedBy string) error {
	return setCostConfig(ctx, m.pool, c, updatedBy)
}

// intToStr avoids importing strconv just for a small helper.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
