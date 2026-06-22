package candidateauth

import "github.com/gofiber/fiber/v2"

// RegisterRoutes mounts the candidate membership API under /api/v1/public/auth.
// The group rides the public rate limiter wired in main.go and bypasses HR Entra
// auth (the /api/v1/public prefix is an unauthed path there).
func RegisterRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/public/auth")
	g.Post("/email/start", h.StartEmail)
	g.Post("/email/verify", h.VerifyEmail)
	g.Post("/logout", h.Logout)

	// Attach the gate per-route (NOT via Group("", mw)) so it does not leak onto
	// the shared /api/v1/public/auth prefix — the Google OAuth routes live under
	// /api/v1/public/auth/google and must stay public.
	gate := RequireCandidate(h.svc, h.cookieName)
	g.Get("/me", gate, h.Me)
	g.Patch("/profile", gate, h.UpdateProfile)
	g.Post("/resume", gate, h.UploadResume) // adds to the library (cap MaxResumes)
	g.Get("/resumes", gate, h.ListResumes)
	g.Get("/resumes/:id/file", gate, h.ViewResume) // short-lived signed URL to open the CV
	g.Post("/resumes/:id/default", gate, h.SetDefaultResume)
	g.Delete("/resumes/:id", gate, h.DeleteResume)
	g.Post("/consent/withdraw", gate, h.WithdrawConsent)
	g.Post("/consent/accept", gate, h.AcceptConsent)
}
