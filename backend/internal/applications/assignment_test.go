package applications

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/nexto/hr-ats/internal/middleware"
	"github.com/nexto/hr-ats/internal/rbac"
	"github.com/nexto/hr-ats/internal/stores"
	"github.com/nexto/hr-ats/pkg/httpx"
)

// assignAppsStub embeds Repository (nil) and overrides only what UpdateAssignment
// calls; any other method would panic, which is fine — it's never reached here.
type assignAppsStub struct {
	Repository
	inScope   bool
	setStore  *int
	setPool   bool
	setCalled bool
}

func (s *assignAppsStub) ExistsInScope(context.Context, uuid.UUID, rbac.Scope) (bool, error) {
	return s.inScope, nil
}
func (s *assignAppsStub) SetAssignment(_ context.Context, _ uuid.UUID, storeNo *int, pool bool) error {
	s.setCalled, s.setStore, s.setPool = true, storeNo, pool
	return nil
}

type assignStoresStub struct{ found bool }

func (s assignStoresStub) FindByNo(_ context.Context, no int) (*stores.Store, error) {
	if !s.found {
		return nil, pgx.ErrNoRows
	}
	return &stores.Store{StoreNo: no, StoreName: "Test Store"}, nil
}

func assignTestApp(apps Repository, st storeReader, role string) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.UserContextKey, middleware.DevUser{ID: uuid.NewString(), Role: role})
		return c.Next()
	})
	h := NewHandler(nil, apps, nil, nil)
	if st != nil {
		h.SetStores(st)
	}
	RegisterRoutes(app, h)
	return app
}

func patchAssignment(t *testing.T, app *fiber.App, body any) (int, *assignAppsStub) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(fiber.MethodPatch, "/api/v1/applications/"+uuid.NewString()+"/assignment", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, nil
}

func TestUpdateAssignment_RoleGate(t *testing.T) {
	apps := &assignAppsStub{inScope: true}
	app := assignTestApp(apps, assignStoresStub{found: true}, "hr_staff") // store-locked → forbidden
	if code, _ := patchAssignment(t, app, map[string]any{"talent_pool": true}); code != fiber.StatusForbidden {
		t.Fatalf("hr_staff should be forbidden, got %d", code)
	}
}

func TestUpdateAssignment_MoveToCentralPool(t *testing.T) {
	apps := &assignAppsStub{inScope: true}
	app := assignTestApp(apps, assignStoresStub{found: true}, "hr_manager")
	code, _ := patchAssignment(t, app, map[string]any{"talent_pool": true})
	if code != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if !apps.setCalled || apps.setStore != nil || !apps.setPool {
		t.Fatalf("expected SetAssignment(nil, true), got store=%v pool=%v called=%v", apps.setStore, apps.setPool, apps.setCalled)
	}
}

func TestUpdateAssignment_AssignToStore(t *testing.T) {
	apps := &assignAppsStub{inScope: true}
	app := assignTestApp(apps, assignStoresStub{found: true}, "sgm")
	code, _ := patchAssignment(t, app, map[string]any{"store_no": 7})
	if code != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if !apps.setCalled || apps.setStore == nil || *apps.setStore != 7 || apps.setPool {
		t.Fatalf("expected SetAssignment(&7, false), got store=%v pool=%v", apps.setStore, apps.setPool)
	}
}

func TestUpdateAssignment_StoreNotFound(t *testing.T) {
	apps := &assignAppsStub{inScope: true}
	app := assignTestApp(apps, assignStoresStub{found: false}, "hr_manager")
	if code, _ := patchAssignment(t, app, map[string]any{"store_no": 999}); code != fiber.StatusNotFound {
		t.Fatalf("expected 404 for missing store, got %d", code)
	}
}

func TestUpdateAssignment_NotInScope(t *testing.T) {
	apps := &assignAppsStub{inScope: false}
	app := assignTestApp(apps, assignStoresStub{found: true}, "hr_manager")
	if code, _ := patchAssignment(t, app, map[string]any{"talent_pool": true}); code != fiber.StatusNotFound {
		t.Fatalf("expected 404 out of scope, got %d", code)
	}
}
