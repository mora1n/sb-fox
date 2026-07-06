package api

import (
	"html"
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
	r.Use(middleware.Compress(5))

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
		http.Error(w, "not found", http.StatusNotFound)
	})
	r.Get("/sub/{token}/*", func(w http.ResponseWriter, r *http.Request) {
		s.handleSubscription(w, r, chi.URLParam(r, "token"), strings.TrimPrefix(chi.URLParam(r, "*"), "/"))
	})
	r.Get("/site.webmanifest", s.handleWebManifest)

	// frontend
	if !s.DevMode && assets.HasDist() {
		if dist, err := assets.DistFS(); err == nil {
			r.NotFound(s.spaHandler(dist))
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
	r.Get("/auth/subscription-token", s.handleGetSubscriptionToken)
	r.Post("/auth/subscription-token/rotate", s.handleRotateSubscriptionToken)

	r.Get("/users", s.handleListUsers)
	r.Post("/users", s.handleCreateUser)
	r.Put("/users/{id}", s.handleUpdateUser)
	r.Delete("/users/{id}", s.handleDeleteUser)
	r.Post("/users/{id}/reset-password", s.handleResetUserPassword)

	r.Get("/settings", s.handleGetSettings)
	r.Put("/settings", s.handleUpdateSettings)
	r.Get("/settings/kernel", s.handleKernelStatus)
	r.Get("/settings/kernels", s.handleListKernels)
	r.Put("/settings/kernels", s.handleSaveKernels)
	r.Post("/settings/kernels/test", s.handleTestKernel)
	r.Get("/kernel/status", s.handlePublicKernelStatus)
	r.Put("/kernel/active", s.handleSetActiveKernel)

	r.Get("/templates", s.handleListTemplates)
	r.Post("/templates", s.handleCreateTemplate)
	r.Get("/templates/by-name", s.handleGetTemplateByName)
	r.Post("/templates/bulk-delete", s.handleBulkDeleteTemplates)
	r.Get("/templates/{id}", s.handleGetTemplate)
	r.Put("/templates/{id}", s.handleUpdateTemplate)
	r.Delete("/templates/{id}", s.handleDeleteTemplate)
	r.Post("/templates/{id}/inspect", s.handleInspectTemplate)
	r.Get("/templates/{id}/structure", s.handleGetTemplateStructure)
	r.Put("/templates/{id}/structure", s.handleUpdateTemplateStructure)
	r.Get("/templates/{id}/export", s.handleExportTemplate)

	r.Get("/nodes", s.handleListNodes)
	r.Post("/nodes", s.handleCreateNode)
	r.Post("/nodes/bulk-delete/preview", s.handlePreviewBulkDeleteNodes)
	r.Post("/nodes/bulk-delete", s.handleBulkDeleteNodes)
	r.Get("/nodes/{id}/usage", s.handleNodeUsage)
	r.Get("/nodes/{id}", s.handleGetNode)
	r.Put("/nodes/{id}", s.handleUpdateNode)
	r.Delete("/nodes/{id}", s.handleDeleteNode)
	r.Post("/nodes/import/links/preview", s.handlePreviewImportLinks)
	r.Post("/nodes/import/links", s.handleImportLinks)
	r.Post("/nodes/import/subscription/preview", s.handlePreviewImportSubscription)
	r.Post("/nodes/import/subscription", s.handleImportSubscription)
	r.Post("/nodes/import/config/preview", s.handlePreviewImportConfig)
	r.Post("/nodes/import/config", s.handleImportConfig)
	r.Post("/nodes/refresh-country", s.handleRefreshCountry)
	r.Post("/nodes/export/template", s.handleExportNodeTemplate)
	r.Post("/nodes/export/links", s.handleExportNodeLinks)

	r.Get("/node-groups", s.handleListNodeGroups)
	r.Post("/node-groups", s.handleCreateNodeGroup)
	r.Post("/node-groups/bulk-delete", s.handleBulkDeleteNodeGroups)
	r.Get("/node-groups/{id}", s.handleGetNodeGroup)
	r.Put("/node-groups/{id}", s.handleUpdateNodeGroup)
	r.Delete("/node-groups/{id}", s.handleDeleteNodeGroup)

	r.Get("/sources", s.handleListSources)
	r.Post("/sources/{id}/refresh", s.handleRefreshSource)
	r.Delete("/sources/{id}", s.handleDeleteSource)

	r.Get("/profiles", s.handleListProfiles)
	r.Post("/profiles", s.handleCreateProfile)
	r.Post("/profiles/bulk-delete", s.handleBulkDeleteProfiles)
	r.Get("/profiles/{id}", s.handleGetProfile)
	r.Put("/profiles/{id}", s.handleUpdateProfile)
	r.Put("/profiles/{id}/subscription-enabled", s.handleSetProfileSubscriptionEnabled)
	r.Delete("/profiles/{id}", s.handleDeleteProfile)

	r.Post("/generate/preview", s.handlePreview)
	r.Post("/generate/validate", s.handleValidate)
	r.Post("/generate/format", s.handleFormat)
}

// spaHandler serves static files from the embedded dist, falling back to
// index.html for client-side routes (SPA history mode).
func (s *Server) spaHandler(dist fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(dist))
	return func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if p == "index.html" {
			s.serveIndex(w, dist)
			return
		}
		if _, err := fs.Stat(dist, p); err != nil {
			// unknown path → serve index.html so the SPA router handles it
			s.serveIndex(w, dist)
			return
		}
		if strings.HasPrefix(p, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, r)
	}
}

func (s *Server) serveIndex(w http.ResponseWriter, dist fs.FS) {
	data, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		http.Error(w, "frontend not built", http.StatusNotFound)
		return
	}
	if displayName, err := s.appDisplayName(); err == nil {
		data = []byte(strings.Replace(string(data), "<title>Loading...</title>", "<title>"+html.EscapeString(displayName)+"</title>", 1))
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

func devPlaceholder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><html><body style="font-family:sans-serif;padding:2rem">
<h1>API is running</h1><p>Frontend is not embedded. Run <code>make frontend</code> for the full UI, or use the Vite dev server.</p>
</body></html>`))
}
