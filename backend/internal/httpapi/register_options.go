package httpapi

import (
	"net/http"

	"bigroup-backend/internal/departments"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/invitations"
)

// registerOptions: GET /api/register/options?code=. Public and rate-limited.
// One generic reason for every non-redeemable code; departments are only
// returned for a valid code (no public org-structure leak).
func (s *Server) registerOptions(w http.ResponseWriter, r *http.Request) {
	code := invitations.NormalizeCode(r.URL.Query().Get("code"))
	if code == "" || len(code) > 64 {
		writeJSON(w, http.StatusOK, dto.RegisterOptions{Valid: false, Reason: "invite_invalid"})
		return
	}
	ok, err := invitations.New(s.pool).IsRedeemable(r.Context(), code)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, dto.RegisterOptions{Valid: false, Reason: "invite_invalid"})
		return
	}
	list, err := departments.New(s.pool).ListActive(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	out := dto.RegisterOptions{Valid: true, Departments: make([]dto.DepartmentRef, 0, len(list))}
	for _, d := range list {
		out.Departments = append(out.Departments, dto.DepartmentRef{ID: d.ID, Name: d.Name})
	}
	writeJSON(w, http.StatusOK, out)
}
