package httpapi

import (
	"net/http"
)

// manageAssessmentGet: GET /api/manage/assessments/{id}, full definition incl. effects.
// Archived assessments are 404.
func (s *Server) manageAssessmentGet(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	a, err := s.manageRepo().Get(r.Context(), id)
	if manageErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, a)
}
