package api

import "net/http"

// handleListSources returns all subscription sources with fetch metadata.
func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListSources()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, list)
}

// handleDeleteSource removes a subscription source.
func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteSource(pathID(r)); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
