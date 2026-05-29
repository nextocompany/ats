package peoplesoft

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/positions"
	"github.com/nexto/hr-ats/internal/vacancies"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// ReengageTrigger enqueues candidate re-engagement when a vacancy opens. It is an
// interface (satisfied by reengage.Trigger) so this package needs no dependency
// on reengage. A nil trigger disables re-engagement.
type ReengageTrigger interface {
	OnVacancyOpened(ctx context.Context, positionID uuid.UUID) error
}

// Handler serves the PeopleSoft webhook + sync endpoints.
type Handler struct {
	vac      vacancies.Repository
	pos      positions.Repository
	svc      *Service
	provider string
	reengage ReengageTrigger
}

// NewHandler builds the PeopleSoft handler. reengage may be nil to disable
// re-engagement on vacancy open.
func NewHandler(vac vacancies.Repository, pos positions.Repository, svc *Service, provider string, reengage ReengageTrigger) *Handler {
	return &Handler{vac: vac, pos: pos, svc: svc, provider: provider, reengage: reengage}
}

type vacancyOpenedReq struct {
	PSVacancyID  string `json:"ps_vacancy_id"`
	StoreID      *int   `json:"store_id"`
	PositionCode string `json:"position_code"`
	Headcount    int    `json:"headcount"`
	OpenedDate   string `json:"opened_date"`
}

// VacancyOpened handles POST /api/v1/ps/vacancy-opened (Direction A).
func (h *Handler) VacancyOpened(c *fiber.Ctx) error {
	var req vacancyOpenedReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	if req.PSVacancyID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "ps_vacancy_id is required")
	}

	// Map PeopleSoft position code → internal position. Unknown codes do not
	// drop the event; the vacancy is stored unmapped for admin attention.
	var positionID *uuid.UUID
	if req.PositionCode != "" {
		if pos, err := h.pos.FindByPSCode(c.UserContext(), req.PositionCode); err == nil {
			positionID = &pos.ID
		} else {
			log.Warn().Str("position_code", req.PositionCode).Str("ps_vacancy_id", req.PSVacancyID).
				Msg("peoplesoft: unmapped position code — storing vacancy unmapped")
		}
	}

	openedAt := time.Now()
	if req.OpenedDate != "" {
		if t, err := time.Parse(dateLayout, req.OpenedDate); err == nil {
			openedAt = t
		}
	}
	headcount := req.Headcount
	if headcount <= 0 {
		headcount = 1
	}

	if err := h.vac.Upsert(c.UserContext(), vacancies.Vacancy{
		PSVacancyID: req.PSVacancyID,
		StoreID:     req.StoreID,
		PositionID:  positionID,
		Headcount:   headcount,
		Status:      "open",
		OpenedAt:    openedAt,
	}); err != nil {
		return err
	}
	// Re-engage matching talent-pool / prior candidates for the opened role
	// (Sprint 5a). Best-effort: a queue failure must not fail the webhook.
	if h.reengage != nil && positionID != nil {
		if err := h.reengage.OnVacancyOpened(c.UserContext(), *positionID); err != nil {
			log.Warn().Err(err).Str("ps_vacancy_id", req.PSVacancyID).Msg("peoplesoft: re-engagement enqueue failed")
		}
	}
	return httpx.OK(c, fiber.Map{"ps_vacancy_id": req.PSVacancyID, "mapped": positionID != nil})
}

type vacancyClosedReq struct {
	PSVacancyID string `json:"ps_vacancy_id"`
	Status      string `json:"status"` // filled | cancelled (default filled)
}

// VacancyClosed handles POST /api/v1/ps/vacancy-closed.
func (h *Handler) VacancyClosed(c *fiber.Ctx) error {
	var req vacancyClosedReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	if req.PSVacancyID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "ps_vacancy_id is required")
	}
	status := req.Status
	if status != "cancelled" {
		status = "filled"
	}
	if err := h.vac.SetStatusByPSID(c.UserContext(), req.PSVacancyID, status); err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"ps_vacancy_id": req.PSVacancyID, "status": status})
}

type syncHiredReq struct {
	ApplicationID string `json:"application_id"`
}

// SyncHired handles POST /api/v1/ps/sync-hired (manual trigger).
func (h *Handler) SyncHired(c *fiber.Ctx) error {
	var req syncHiredReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	id, err := uuid.Parse(req.ApplicationID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "valid application_id is required")
	}
	if err := h.svc.SyncHired(c.UserContext(), id); err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"application_id": req.ApplicationID, "synced": true})
}

// Health handles GET /api/v1/ps/health.
func (h *Handler) Health(c *fiber.Ctx) error {
	return httpx.OK(c, fiber.Map{"provider": h.provider, "status": "ok"})
}
