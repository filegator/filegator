package app

import (
	"net/http"

	"github.com/filegator/filegator/internal/api"
	"github.com/filegator/filegator/internal/api/admin"
	"github.com/filegator/filegator/internal/router"
)

// RegisterAPIRoutes adds every /api route to rt.
//
// It is the single list of the endpoints the server exposes — the routes
// registered by the api and admin packages. routes() uses it at startup, and
// internal/apidoc uses it to generate the endpoint tables in
// docs/content/api.md, so the documentation cannot drift from the router.
func RegisterAPIRoutes(rt *router.Router, s *api.Server) {
	s.AuthRoutes(rt)
	s.SSOAuthRoutes(rt)
	s.Routes(rt)
	s.WriteRoutes(rt)
	s.UploadRoutes(rt)
	s.EditRoutes(rt)
	s.ArchiveRoutes(rt)
	s.SearchRoutes(rt)
	s.CapabilitiesRoutes(rt)
	admin.New(s).Routes(rt)
}

// registerFunctionRoutes adds the endpoints whose handlers need the App: the
// health check, the language files and the public browser configuration.
func (a *App) registerFunctionRoutes(rt *router.Router) {
	rt.Get("/api/ping", pingHandler)
	rt.Get("/api/i18n/{lang}", a.serveLanguage)
	rt.Get("/api/i18n", a.listLanguages)
	rt.Get("/api/config", a.serveConfig)
	RegisterAPIRoutes(rt, a.api)
}

// pingHandler answers the health check, used by tests and monitoring.
func pingHandler(w http.ResponseWriter, r *http.Request, _ router.Params) {
	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
