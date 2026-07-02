package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mora1n/sb-fox/internal/assets"
)

// Router builds the full HTTP handler: guarded /api, public /sub, and the
// embedded SPA (unless in dev mode).
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	r.Route("/api", func(api chi.Router) {
		// public bootstrap/auth endpoints
		api.Get("/app", s.handleAppInfo)
		api.Post("/auth/login", s.handleLogin)
		api.Post("/auth/register", s.handleRegister)

		// authenticated endpoints
		api.Group(func(pr chi.Router) {
			pr.Use(s.requireAuth)
			s.mountAuthed(pr)
		})
	})

	// public subscription (unauthenticated by design)
	r.Get("/sub/{token}", func(w http.ResponseWriter, r *http.Request) {
		s.handleSubscription(w, r, chi.URLParam(r, "token"))
	})

	// frontend
	if !s.DevMode && assets.HasDist() {
		if dist, err := assets.DistFS(); err == nil {
			r.NotFound(spaHandler(dist))
		}
	} else {
		r.NotFound(devPlaceholder)
	}
	return r
}

// mountAuthed registers all endpoints requiring authentication.
func (s *Server) mountAuthed(r chi.Router) {
	r.Post("/auth/logout", s.handleLogout)
	r.Get("/auth/me", s.handleMe)
	r.Post("/auth/password", s.handleChangePassword)

	r.Get("/users", s.handleListUsers)
	r.Post("/users", s.handleCreateUser)
	r.Put("/users/{id}", s.handleUpdateUser)
	r.Delete("/users/{id}", s.handleDeleteUser)
	r.Post("/users/{id}/reset-password", s.handleResetUserPassword)

	r.Get("/settings", s.handleGetSettings)
	r.Put("/settings", s.handleUpdateSettings)
	r.Get("/settings/kernel", s.handleKernelStatus)

	r.Get("/templates", s.handleListTemplates)
	r.Post("/templates", s.handleCreateTemplate)
	r.Get("/templates/{id}", s.handleGetTemplate)
	r.Put("/templates/{id}", s.handleUpdateTemplate)
	r.Delete("/templates/{id}", s.handleDeleteTemplate)
	r.Post("/templates/{id}/inspect", s.handleInspectTemplate)

	r.Get("/nodes", s.handleListNodes)
	r.Post("/nodes", s.handleCreateNode)
	r.Get("/nodes/{id}", s.handleGetNode)
	r.Put("/nodes/{id}", s.handleUpdateNode)
	r.Delete("/nodes/{id}", s.handleDeleteNode)
	r.Post("/nodes/import/links", s.handleImportLinks)
	r.Post("/nodes/import/subscription", s.handleImportSubscription)
	r.Post("/nodes/import/config", s.handleImportConfig)
	r.Post("/nodes/refresh-country", s.handleRefreshCountry)
	r.Post("/nodes/export/template", s.handleExportNodeTemplate)

	r.Get("/sources", s.handleListSources)
	r.Post("/sources/{id}/refresh", s.handleRefreshSource)
	r.Delete("/sources/{id}", s.handleDeleteSource)

	r.Get("/profiles", s.handleListProfiles)
	r.Post("/profiles", s.handleCreateProfile)
	r.Get("/profiles/{id}", s.handleGetProfile)
	r.Put("/profiles/{id}", s.handleUpdateProfile)
	r.Delete("/profiles/{id}", s.handleDeleteProfile)
	r.Post("/profiles/{id}/rotate-token", s.handleRotateToken)

	r.Post("/generate/preview", s.handlePreview)
	r.Post("/generate/validate", s.handleValidate)
	r.Post("/generate/format", s.handleFormat)
}

// spaHandler serves static files from the embedded dist, falling back to
// index.html for client-side routes (SPA history mode).
func spaHandler(dist fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(dist))
	return func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(dist, p); err != nil {
			// unknown path → serve index.html so the SPA router handles it
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			serveIndex(w, r2, dist)
			return
		}
		fileServer.ServeHTTP(w, r)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request, dist fs.FS) {
	data, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		http.Error(w, "frontend not built", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func devPlaceholder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><html><body style="font-family:sans-serif;padding:2rem">
<h1>sb-fox</h1><p>API is running. Frontend not embedded (dev mode). Run <code>make frontend</code> for the full UI, or use the Vite dev server.</p>
</body></html>`))
}
