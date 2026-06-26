package executive

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service produces the executive overview + Recruitment ROI payloads.
type Service interface {
	Overview(ctx context.Context) (Overview, error)
	ROI(ctx context.Context, f ExecFilters) (ROIView, error)
	// GetCostConfig / SetCostConfig manage the admin ROI cost assumptions. They are
	// DB-backed regardless of provider so the cost editor works in mock + live.
	GetCostConfig(ctx context.Context) (CostConfig, error)
	SetCostConfig(ctx context.Context, c CostConfig, updatedBy string) error
}

// NewService selects the mock or live implementation based on provider.
// Anything other than "real" falls back to "mock" so the demo stays the safe
// default (mirrors the AI/PS/Graph provider seams in pkg/config).
func NewService(pool *pgxpool.Pool, provider string) Service {
	if provider == "real" {
		return &liveService{pool: pool}
	}
	return &mockService{pool: pool}
}
