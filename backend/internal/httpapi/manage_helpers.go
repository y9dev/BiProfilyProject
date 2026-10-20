package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"bigroup-backend/internal/assessments"
	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/characteristics"
	"bigroup-backend/internal/departments"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/invitations"
	"bigroup-backend/internal/model"
)

// Helpers shared by the 5B manager/admin handlers (prefix mgr to stay clear of other steps).

func mgrError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, dto.Error{Error: code})
}

func mgrFieldErrors(w http.ResponseWriter, errs ...dto.FieldError) {
	writeJSON(w, http.StatusUnprocessableEntity, dto.Error{Error: "validation", Errors: errs})
}

func mgrID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// mgrDecode reads a JSON body (max 64 KiB); false means 400 was written.
func mgrDecode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(v); err != nil {
		mgrError(w, http.StatusBadRequest, "bad_json")
		return false
	}
	return true
}

// mgrName validates a trimmed name of 1..max runes.
func mgrName(s string, max int) (string, *dto.FieldError) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return s, &dto.FieldError{Path: "name", Code: "required"}
	case utf8.RuneCountInString(s) > max:
		return s, &dto.FieldError{Path: "name", Code: "too_long"}
	}
	return s, nil
}

func (s *Server) mgrAudit(r *http.Request, action, entityType string, id int64) {
	sess := auth.UserOf(r.Context())
	s.audit(r, model.AuditEntry{ActorID: &sess.UserID, ActorRole: sess.Role, Action: action, EntityType: entityType, EntityID: itoa(id), Result: "ok"})
}

// Repositories are built from the pool on demand so Deps and main.go stay untouched.
func (s *Server) invRepo() invitations.Repository {
	if s.invitations != nil {
		return s.invitations
	}
	return invitations.New(s.pool)
}
func (s *Server) deptRepo() departments.Repository     { return departments.New(s.pool) }
func (s *Server) charRepo() characteristics.Repository { return characteristics.New(s.pool) }
func (s *Server) manageRepo() *assessments.ManageRepo  { return assessments.NewManageRepo(s.pool) }
