package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"bigroup-backend/internal/assessments"
	"bigroup-backend/internal/httpapi/dto"
)

// manageErr maps repo errors; true when a response was written.
func manageErr(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, assessments.ErrNotFound):
		mgrError(w, http.StatusNotFound, "not_found")
	default:
		writeErr(w, http.StatusInternalServerError, "db error")
	}
	return true
}

func (s *Server) manageAssessmentsList(w http.ResponseWriter, r *http.Request) {
	list, err := s.manageRepo().List(r.Context())
	if manageErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// manageAssessmentsPatch edits title/description only; structure is immutable.
func (s *Server) manageAssessmentsPatch(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	var in dto.AssessmentMetaRequest
	if !mgrDecode(w, r, &in) {
		return
	}
	var errs []dto.FieldError
	if in.Title != nil {
		n := utf8.RuneCountInString(strings.TrimSpace(*in.Title))
		if n == 0 {
			errs = append(errs, dto.FieldError{Path: "title", Code: assessments.CodeRequired})
		} else if n > 200 {
			errs = append(errs, dto.FieldError{Path: "title", Code: assessments.CodeTooLong})
		} else {
			t := strings.TrimSpace(*in.Title)
			in.Title = &t
		}
	}
	if in.Description != nil && utf8.RuneCountInString(*in.Description) > 4000 {
		errs = append(errs, dto.FieldError{Path: "description", Code: assessments.CodeTooLong})
	}
	if len(errs) > 0 {
		mgrFieldErrors(w, errs...)
		return
	}
	it, err := s.manageRepo().UpdateMeta(r.Context(), id, in.Title, in.Description)
	if manageErr(w, err) {
		return
	}
	s.mgrAudit(r, "assessment_update", "assessment", id)
	writeJSON(w, http.StatusOK, it)
}

func (s *Server) manageAssessmentsVisibility(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	var in map[string]json.RawMessage
	if !mgrDecode(w, r, &in) {
		return
	}
	var hidden bool
	if v, present := in["hidden"]; !present || json.Unmarshal(v, &hidden) != nil {
		mgrFieldErrors(w, dto.FieldError{Path: "hidden", Code: assessments.CodeRequired})
		return
	}
	it, err := s.manageRepo().SetHidden(r.Context(), id, hidden)
	if manageErr(w, err) {
		return
	}
	s.mgrAudit(r, "assessment_visibility", "assessment", id)
	writeJSON(w, http.StatusOK, it)
}

func (s *Server) manageAssessmentsExpiry(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	var in dto.ExpiryRequest
	if !mgrDecode(w, r, &in) {
		return
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
		mgrFieldErrors(w, dto.FieldError{Path: "expiresAt", Code: assessments.CodePastExpiry})
		return
	}
	it, err := s.manageRepo().SetExpiry(r.Context(), id, in.ExpiresAt)
	if manageErr(w, err) {
		return
	}
	s.mgrAudit(r, "assessment_expiry", "assessment", id)
	writeJSON(w, http.StatusOK, it)
}

// manageAssessmentsDelete: hard delete without attempts, archive with attempts. Both 204.
func (s *Server) manageAssessmentsDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	archived, err := s.manageRepo().Delete(r.Context(), id)
	if manageErr(w, err) {
		return
	}
	action := "assessment_delete"
	if archived {
		action = "assessment_archive"
	}
	s.mgrAudit(r, action, "assessment", id)
	w.WriteHeader(http.StatusNoContent)
}
