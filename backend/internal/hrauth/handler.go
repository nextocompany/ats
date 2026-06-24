package hrauth

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// passwordPolicyMsg is the user-facing explanation for a rejected password.
const passwordPolicyMsg = "password must be 10–72 characters and include a letter and a number"

// Handler serves the HR password sign-in and super_admin account management API.
type Handler struct {
	svc        *Service
	cookieName string
	secure     bool // Secure + SameSite=None in prod (cross-site dashboard↔api)
}

// NewHandler builds the hrauth handler. secure should be true outside development
// so the session cookie is sent on cross-site dashboard requests.
func NewHandler(svc *Service, secure bool) *Handler {
	return &Handler{svc: svc, cookieName: middleware.HRAuthCookieName, secure: secure}
}

// RegisterRoutes mounts the login/logout endpoints (unauthenticated) and the
// super_admin user-management endpoints (gated in-handler).
func RegisterRoutes(app *fiber.App, h *Handler) {
	app.Post("/api/v1/auth/login", h.Login)
	app.Post("/api/v1/auth/logout", h.Logout)
	app.Get("/api/v1/admin/users", h.ListUsers)
	app.Post("/api/v1/admin/users", h.CreateUser)
	app.Patch("/api/v1/admin/users/:id", h.UpdateUser)
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login handles POST /api/v1/auth/login. On success it sets the httpOnly session
// cookie and returns the user projection. All failures return a single generic
// 401 so the response never reveals whether the email exists.
func (h *Handler) Login(c *fiber.Ctx) error {
	var req loginReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	tok, expires, user, err := h.svc.Login(c.UserContext(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return httpx.Fail(c, fiber.StatusUnauthorized, "invalid email or password")
		}
		log.Error().Err(err).Msg("hrauth: login failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "login failed")
	}
	writeAuthCookie(c, h.cookieName, h.secure, tok, expires)
	return httpx.OK(c, user)
}

// Logout handles POST /api/v1/auth/logout. It revokes the session behind the
// cookie (idempotent) and clears it. Unauthenticated by design so a stale cookie
// still clears cleanly.
func (h *Handler) Logout(c *fiber.Ctx) error {
	_ = h.svc.Logout(c.UserContext(), c.Cookies(h.cookieName))
	clearAuthCookie(c, h.cookieName, h.secure)
	return httpx.OK(c, fiber.Map{"ok": true})
}

// requireSuperAdmin returns the authenticated user and whether they may manage
// accounts. Account provisioning is the highest-privilege operation, so it is
// restricted to super_admin (mirrors the system-settings gate).
func requireSuperAdmin(c *fiber.Ctx) bool {
	u, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	return rbac.Can(u.Role, rbac.PermUsersAdmin)
}

// ListUsers handles GET /api/v1/admin/users.
func (h *Handler) ListUsers(c *fiber.Ctx) error {
	if !requireSuperAdmin(c) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage users")
	}
	users, err := h.svc.ListUsers(c.UserContext())
	if err != nil {
		log.Error().Err(err).Msg("hrauth: list users failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "could not list users")
	}
	return httpx.OK(c, users)
}

type createUserReq struct {
	Email     string `json:"email"`
	FullName  string `json:"full_name"`
	Role      string `json:"role"`
	StoreID   *int   `json:"store_id"`
	Subregion string `json:"subregion"`
	Password  string `json:"password"`
}

// CreateUser handles POST /api/v1/admin/users.
func (h *Handler) CreateUser(c *fiber.Ctx) error {
	if !requireSuperAdmin(c) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage users")
	}
	var req createUserReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	user, err := h.svc.CreateUser(c.UserContext(), NewUserInput(req))
	if err != nil {
		return h.writeUserError(c, err)
	}
	return httpx.Created(c, user)
}

type updateUserReq struct {
	FullName     *string `json:"full_name"`
	Role         *string `json:"role"`
	StoreID      *int    `json:"store_id"`
	Subregion    *string `json:"subregion"`
	IsActive     *bool   `json:"is_active"`
	Phone        *string `json:"phone"`
	IsDPO        *bool   `json:"is_dpo"`
	IsPrimaryDPO *bool   `json:"is_primary_dpo"`
	Password     *string `json:"password"`
}

// UpdateUser handles PATCH /api/v1/admin/users/:id.
func (h *Handler) UpdateUser(c *fiber.Ctx) error {
	if !requireSuperAdmin(c) {
		return httpx.Fail(c, fiber.StatusForbidden, "insufficient role to manage users")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid user id")
	}
	var req updateUserReq
	if err := c.BodyParser(&req); err != nil {
		return httpx.Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	caller, _ := c.Locals(middleware.UserContextKey).(middleware.DevUser)
	user, err := h.svc.UpdateUser(c.UserContext(), id, caller.ID, UpdateUserInput(req))
	if err != nil {
		return h.writeUserError(c, err)
	}
	return httpx.OK(c, user)
}

// writeUserError maps a provisioning error to the right status + message.
func (h *Handler) writeUserError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrEmailExists):
		return httpx.Fail(c, fiber.StatusConflict, "a user with that email already exists")
	case errors.Is(err, ErrInvalidRole):
		return httpx.Fail(c, fiber.StatusBadRequest, "unknown role")
	case errors.Is(err, ErrSelfLockout):
		return httpx.Fail(c, fiber.StatusBadRequest, "you cannot disable or demote your own account")
	case errors.Is(err, ErrWeakPassword):
		return httpx.Fail(c, fiber.StatusBadRequest, passwordPolicyMsg)
	case errors.Is(err, ErrInvalidCredentials):
		return httpx.Fail(c, fiber.StatusBadRequest, "email is required")
	case errors.Is(err, ErrNotFound):
		return httpx.Fail(c, fiber.StatusNotFound, "user not found")
	default:
		log.Error().Err(err).Msg("hrauth: user provisioning failed")
		return httpx.Fail(c, fiber.StatusInternalServerError, "operation failed")
	}
}

// --- cookie helpers --------------------------------------------------------

// cookieSameSite mirrors the candidate session: "None" in prod so the cookie is
// sent on cross-site dashboard→api fetches (azurecontainerapps.io is a public
// suffix → dashboard and api are cross-site), "Lax" in local dev.
func cookieSameSite(secure bool) string {
	if secure {
		return "None"
	}
	return "Lax"
}

func writeAuthCookie(c *fiber.Ctx, name string, secure bool, token string, expires time.Time) {
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    token,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: cookieSameSite(secure),
		Expires:  expires,
		Path:     "/",
	})
}

func clearAuthCookie(c *fiber.Ctx, name string, secure bool) {
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    "",
		HTTPOnly: true,
		Secure:   secure,
		SameSite: cookieSameSite(secure),
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Path:     "/",
	})
}
