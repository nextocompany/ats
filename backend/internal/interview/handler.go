package interview

import (
	"context"
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// ScopeChecker reports whether an application is visible to an RBAC scope. The
// applications repository satisfies it; it keeps per-record authorization on the
// HR interview endpoints consistent with the list-level scoping used elsewhere.
type ScopeChecker interface {
	ExistsInScope(ctx context.Context, id uuid.UUID, scope rbac.Scope) (bool, error)
}

// Handler exposes the public (candidate-facing) and dashboard (HR-facing)
// interview endpoints.
type Handler struct {
	svc           *Service
	scoper        ScopeChecker
	portalBaseURL string
}

// NewHandler builds the interview HTTP handler.
func NewHandler(svc *Service, scoper ScopeChecker, portalBaseURL string) *Handler {
	return &Handler{svc: svc, scoper: scoper, portalBaseURL: portalBaseURL}
}

// scopeFrom derives the caller's RBAC scope from the authenticated-user locals.
func scopeFrom(c *fiber.Ctx) rbac.Scope {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	return rbac.New(u.Role, u.StoreID, u.Subregion)
}

// authorizeApplication returns a 404 (not 403, to avoid leaking existence) when
// the caller's scope cannot see the application.
func (h *Handler) authorizeApplication(c *fiber.Ctx, id uuid.UUID) error {
	if h.scoper == nil {
		return nil
	}
	ok, err := h.scoper.ExistsInScope(c.UserContext(), id, scopeFrom(c))
	if err != nil {
		return err
	}
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "application not found")
	}
	return nil
}

// publicTurn is the candidate-safe projection of a conversation turn.
type publicTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// publicSession is what the candidate-facing chat sees — no token, no evaluation.
type publicSession struct {
	Status string       `json:"status"`
	Turns  []publicTurn `json:"turns"`
	Done   bool         `json:"done"`
}

func toPublicSession(s *Session) publicSession {
	turns := make([]publicTurn, 0, len(s.Conversation))
	for _, t := range s.Conversation {
		turns = append(turns, publicTurn{Role: t.Role, Content: t.Content})
	}
	return publicSession{Status: s.Status, Turns: turns, Done: s.Status == StatusCompleted}
}

// Start handles GET /api/v1/public/interview/:token — loads or seeds the chat.
func (h *Handler) Start(c *fiber.Ctx) error {
	session, err := h.svc.Start(c.UserContext(), c.Params("token"))
	if err != nil {
		return mapError(err)
	}
	return httpx.OK(c, toPublicSession(session))
}

type respondReq struct {
	Content string `json:"content"`
}

// Respond handles POST /api/v1/public/interview/:token/message.
func (h *Handler) Respond(c *fiber.Ctx) error {
	var req respondReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	session, err := h.svc.Respond(c.UserContext(), c.Params("token"), req.Content)
	if err != nil {
		return mapError(err)
	}
	return httpx.OK(c, toPublicSession(session))
}

// Invite handles POST /api/v1/applications/:id/interview (HR action).
func (h *Handler) Invite(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	if err := h.authorizeApplication(c, id); err != nil {
		return err
	}
	session, err := h.svc.Invite(c.UserContext(), id)
	if err != nil {
		return mapError(err)
	}
	return httpx.OK(c, fiber.Map{
		"id":            session.ID,
		"status":        session.Status,
		"access_token":  session.AccessToken,
		"interview_url": h.interviewURL(session.AccessToken),
	})
}

// Get handles GET /api/v1/applications/:id/interview (HR view of the session).
func (h *Handler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	if err := h.authorizeApplication(c, id); err != nil {
		return err
	}
	session, err := h.svc.Get(c.UserContext(), id)
	if err != nil {
		return mapError(err)
	}
	if session == nil {
		return fiber.NewError(fiber.StatusNotFound, "no interview for this application")
	}
	url := h.interviewURL(session.AccessToken)
	// Don't echo the raw token as its own field — the interview_url carries it for
	// sharing, and a bare token in response bodies is easy to leak via logs.
	session.AccessToken = ""
	return httpx.OK(c, fiber.Map{
		"session":       session,
		"interview_url": url,
	})
}

func (h *Handler) interviewURL(token string) string {
	// Token in the URL fragment (#) so it never reaches the server / proxy logs.
	return fmt.Sprintf("%s/interview#token=%s", h.portalBaseURL, token)
}

// mapError translates service/repository errors into HTTP errors.
func mapError(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return fiber.NewError(fiber.StatusNotFound, "interview not found")
	case errors.Is(err, ErrNotAnswerable):
		return fiber.NewError(fiber.StatusConflict, "interview is no longer open")
	case errors.Is(err, ErrConflict):
		return fiber.NewError(fiber.StatusConflict, "interview was updated concurrently, please retry")
	case errors.Is(err, ErrEmptyAnswer):
		return fiber.NewError(fiber.StatusBadRequest, "answer must not be empty")
	case errors.Is(err, ErrNotScreened):
		return fiber.NewError(fiber.StatusConflict, "AI interview is only available after screening")
	default:
		return err
	}
}
