package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"bigroup-backend/internal/assessments"
	"bigroup-backend/internal/attempts"
	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
)

const maxSubmitBody = 1 << 20

// registerEmployeeRoutes: profile, visible assessments, take, submit, own attempt result.
// All use routeOpts{} (any signed-in user, avatar gate on).
func (s *Server) registerEmployeeRoutes() {
	h := &employeeHandlers{s: s, assess: assessments.NewEmployeeRepo(s.pool), attempts: attempts.New(s.pool)}
	s.protected("GET /api/me/profile", h.profile, routeOpts{})
	s.protected("GET /api/assessments", h.list, routeOpts{})
	s.protected("GET /api/assessments/{id}/take", h.take, routeOpts{})
	s.protected("POST /api/assessments/{id}/submit", h.submit, routeOpts{})
	s.protected("GET /api/me/attempts/{id}", h.attemptResult, routeOpts{})
}

type employeeHandlers struct {
	s        *Server
	assess   *assessments.EmployeeRepo
	attempts *attempts.Repo
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func (h *employeeHandlers) list(w http.ResponseWriter, r *http.Request) {
	sess := auth.UserOf(r.Context())
	items, err := h.assess.ListVisible(r.Context(), sess.UserID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *employeeHandlers) take(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	sess := auth.UserOf(r.Context())
	v, err := h.assess.Take(r.Context(), sess.UserID, id)
	switch {
	case errors.Is(err, assessments.ErrNotVisible):
		writeErr(w, http.StatusNotFound, "not_found")
	case errors.Is(err, assessments.ErrAlreadyCompleted):
		writeErr(w, http.StatusConflict, "already_completed")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "db error")
	default:
		writeJSON(w, http.StatusOK, v)
	}
}

func (h *employeeHandlers) submit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	var in dto.SubmitRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSubmitBody)).Decode(&in); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, dto.Error{Error: "validation_failed",
			Errors: []dto.FieldError{{Path: "answers", Code: "invalid_body"}}})
		return
	}
	sess := auth.UserOf(r.Context())
	entry := model.AuditEntry{ActorID: &sess.UserID, ActorRole: sess.Role, EntityType: "assessment", EntityID: itoa(id)}

	res, err := h.attempts.Submit(r.Context(), sess.UserID, id, in.Answers)
	var verr *attempts.ValidationError
	switch {
	case errors.Is(err, attempts.ErrNotVisible):
		writeErr(w, http.StatusNotFound, "not_found")
	case errors.Is(err, attempts.ErrAlreadyCompleted):
		entry.Action, entry.Result = "attempt_duplicate", "deny"
		h.s.audit(r, entry)
		writeErr(w, http.StatusConflict, "already_completed")
	case errors.As(err, &verr):
		writeJSON(w, http.StatusUnprocessableEntity, dto.Error{Error: "validation_failed", Errors: verr.Errors})
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "db error")
	default:
		entry.Action, entry.Result = "attempt_submit", "ok"
		h.s.audit(r, entry)
		writeJSON(w, http.StatusCreated, res)
	}
}
