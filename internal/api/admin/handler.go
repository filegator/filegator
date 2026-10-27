// Package admin implements the administrator-only API under /api/admin/.
//
// It wraps the core API server so both share one configuration store, one auth
// provider and one storage backend. Layering: internal/api/httpx holds the
// transport helpers, internal/api holds the user-facing API, and this package
// adds the admin endpoints on top of it.
package admin

import (
	"net/http"

	"github.com/filegator/filegator/internal/api"
	"github.com/filegator/filegator/internal/api/httpx"
	"github.com/filegator/filegator/internal/router"
)

// Handler serves the administrator endpoints.
type Handler struct {
	// Core is the shared API server: config, users, sessions and storage.
	Core *api.Server
}

// New wraps an API server.
func New(core *api.Server) *Handler { return &Handler{Core: core} }

// adminOnly wraps a handler so that only administrators reach it.
func (h *Handler) adminOnly(fn func(http.ResponseWriter, *http.Request, router.Params)) router.Handler {
	return func(w http.ResponseWriter, r *http.Request, p router.Params) {
		u, err := h.Core.CurrentUser(r)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		if !u.IsAdmin() {
			httpx.WriteError(w, httpx.Errorf(http.StatusForbidden, httpx.CodeForbidden,
				"administrator access required"))
			return
		}
		if r.Method != http.MethodGet {
			if err := h.Core.RequireCSRF(r); err != nil {
				httpx.WriteError(w, err)
				return
			}
		}
		fn(w, r, p)
	}
}

// Routes registers every administrator endpoint.
func (h *Handler) Routes(rt *router.Router) {
	// Storage
	rt.Get("/api/admin/browse", h.adminOnly(h.BrowseHost))
	rt.Get("/api/admin/storage/browse", h.adminOnly(h.BrowseStorage))
	rt.Get("/api/admin/root", h.adminOnly(h.GetRoot))
	rt.Post("/api/admin/root", h.adminOnly(h.SetRoot))
	rt.Get("/api/admin/storage", h.adminOnly(h.GetStorage))
	rt.Post("/api/admin/storage", h.adminOnly(h.SetStorage))

	// Status and settings
	rt.Get("/api/admin/status", h.adminOnly(h.Status))
	rt.Get("/api/admin/settings", h.adminOnly(h.GetSettings))
	rt.Post("/api/admin/settings", h.adminOnly(h.SetSettings))

	// Authentication
	rt.Get("/api/admin/auth", h.adminOnly(h.GetAuth))
	rt.Post("/api/admin/auth", h.adminOnly(h.SetAuth))

	// Users
	rt.Get("/api/admin/users", h.adminOnly(h.ListUsers))
	rt.Post("/api/admin/users", h.adminOnly(h.CreateUser))
	rt.Post("/api/admin/users/{username}", h.adminOnly(h.UpdateUser))
	rt.Post("/api/admin/users/{username}/delete", h.adminOnly(h.DeleteUser))
	rt.Post("/api/admin/users/{username}/password", h.adminOnly(h.SetPassword))
}
