// Package branch implements F04 (Step 6): map a candidate's province to a
// subregion, find open vacancies for the position there, and assign the nearest
// store — or route to the talent pool when none exist.
package branch

import (
	"context"
	"math"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/stores"
	"github.com/nexto/hr-ats/internal/vacancies"
)

// Assignment is the outcome of branch assignment.
type Assignment struct {
	StoreNo    *int
	VacancyID  *uuid.UUID // the open vacancy the candidate was matched to (nil for talent pool)
	TalentPool bool
	Subregion  string
}

// Assigner ranks open vacancies by distance from the candidate's province.
type Assigner struct {
	vac vacancies.Repository
}

// NewAssigner wires the assigner.
func NewAssigner(vac vacancies.Repository) *Assigner {
	return &Assigner{vac: vac}
}

// Assign resolves the subregion and picks the nearest store with an open
// vacancy. formatTypes (from the position) optionally restrict eligible stores.
func (a *Assigner) Assign(ctx context.Context, province string, positionID uuid.UUID, formatTypes []string) (Assignment, error) {
	subregion := stores.ResolveSubregion(province)
	if subregion == "" {
		return Assignment{TalentPool: true}, nil // unknown province → talent pool
	}

	open, err := a.vac.FindOpen(ctx, subregion, positionID)
	if err != nil {
		return Assignment{}, err
	}
	open = filterByFormat(open, formatTypes)
	if len(open) == 0 {
		return Assignment{TalentPool: true, Subregion: subregion}, nil
	}

	cLat, cLng, hasCentroid := stores.ProvinceCentroid(province)

	bestIdx := 0
	bestDist := math.MaxFloat64
	for i, v := range open {
		dist := math.MaxFloat64 - 1 // stores without coords sort last but remain eligible
		if hasCentroid && v.StoreLat != nil && v.StoreLng != nil {
			dist = haversineKM(cLat, cLng, *v.StoreLat, *v.StoreLng)
		}
		if dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}

	best := open[bestIdx]
	storeNo := best.StoreNo
	vacancyID := best.ID
	return Assignment{StoreNo: &storeNo, VacancyID: &vacancyID, Subregion: subregion}, nil
}

// LocationScore returns 0–20 for the scoring engine: full marks when the
// candidate's subregion has an open vacancy for the position, partial when the
// position has vacancies elsewhere, zero otherwise.
func (a *Assigner) LocationScore(ctx context.Context, province string, positionID uuid.UUID) (int, error) {
	subregion := stores.ResolveSubregion(province)
	if subregion != "" {
		open, err := a.vac.FindOpen(ctx, subregion, positionID)
		if err != nil {
			return 0, err
		}
		if len(open) > 0 {
			return 20, nil
		}
	}
	total, err := a.vac.CountOpenForPosition(ctx, positionID)
	if err != nil {
		return 0, err
	}
	if total > 0 {
		return 8, nil // vacancies exist, just not nearby
	}
	return 0, nil
}

// filterByFormat keeps vacancies whose store format is compatible with the
// position's format types. When either side is empty, no filtering applies.
func filterByFormat(open []vacancies.OpenVacancy, formatTypes []string) []vacancies.OpenVacancy {
	if len(formatTypes) == 0 {
		return open
	}
	allowed := make(map[string]struct{}, len(formatTypes))
	for _, f := range formatTypes {
		allowed[f] = struct{}{}
	}
	var out []vacancies.OpenVacancy
	for _, v := range open {
		if v.FormatType == "" {
			out = append(out, v) // unknown format — don't exclude
			continue
		}
		if _, ok := allowed[v.FormatType]; ok {
			out = append(out, v)
		}
	}
	return out
}

// haversineKM returns the great-circle distance between two coordinates in km.
func haversineKM(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKM = 6371.0
	dLat := rad(lat2 - lat1)
	dLng := rad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return earthRadiusKM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func rad(deg float64) float64 { return deg * math.Pi / 180 }
