package httpapi

// Profile handlers (GET /api/me/profile mean + timeline, GET /api/me/attempts/{id}
// owner-only detail). Routes are registered in assessments_employee.go; aggregation
// lives in attempts.Repo on top of internal/scoring.

import (
	"errors"
	"net/http"

	"bigroup-backend/internal/attempts"
	"bigroup-backend/internal/auth"
)

func (h *employeeHandlers) profile(w http.ResponseWriter, r *http.Request) {
	sess := auth.UserOf(r.Context())
	p, err := h.attempts.Profile(r.Context(), sess.UserID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	p.Stats.Available, err = h.assess.CountAvailable(r.Context(), sess.UserID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *employeeHandlers) attemptResult(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	sess := auth.UserOf(r.Context())
	res, err := h.attempts.Result(r.Context(), sess.UserID, id)
	switch {
	case errors.Is(err, attempts.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "db error")
	default:
		writeJSON(w, http.StatusOK, res)
	}
}
