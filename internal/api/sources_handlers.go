package api

import (
	"net/http"

	"github.com/mora1n/sb-fox/internal/store"
)

// handleListSources returns all subscription sources with fetch metadata.
func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	list, err := s.Store.ListSources(ownerID, allOwners)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, list)
}

// handleDeleteSource removes a subscription source.
func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	src, err := s.Store.GetSourceForUser(pathID(r), ownerID, allOwners)
	if err != nil {
		respondError(w, http.StatusNotFound, "not_found", "source not found")
		return
	}
	if err := s.Store.DeleteSourceForUser(src.ID, src.OwnerUserID); err != nil {
		if err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "source not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
