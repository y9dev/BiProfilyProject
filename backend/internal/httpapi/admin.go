// Package httpapi — административные эндпоинты (только роль admin).
package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"bigroup-backend/internal/admin"
	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
)

// adminListMaxLimit caps ?limit= on admin lists.
const adminListMaxLimit = 200

func (s *Server) adminUsersList(w http.ResponseWriter, r *http.Request) {
	sess := auth.UserOf(r.Context())
	query := strings.TrimSpace(r.URL.Query().Get("search"))
	role := strings.TrimSpace(r.URL.Query().Get("role"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}
	limit = min(limit, adminListMaxLimit)
	list, total, err := s.admin.UsersSearch(r.Context(), query, role, status, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	out := dto.AdminUserList{Items: make([]dto.AdminUser, 0, len(list)), Total: total}
	for _, u := range list {
		out.Items = append(out.Items, dto.AdminUser{
			ID: u.ID, Login: u.Login, FullName: u.FullName, JobTitle: u.JobTitle,
			Department: deptRef(u.DepartmentID, u.DepartmentName), Role: u.Role, Status: u.Status, CreatedAt: u.CreatedAt,
		})
	}
	w.Header().Set("X-Total-Count", itoa(int64(total)))
	writeJSON(w, http.StatusOK, out)
	sessRole := sess.Role
	s.audit(r, model.AuditEntry{ActorID: &sess.UserID, ActorRole: sessRole, Action: "admin_users_list", EntityType: "user", Result: "ok"})
}

func (s *Server) registerAdminRoutes() {
	adm := routeOpts{roles: rolesAdmin}
	s.protected("GET /api/admin/users", s.adminUsersList, adm)
	s.protected("PUT /api/admin/users/{id}/role", s.adminSetRole, adm)
	s.protected("POST /api/admin/users/{id}/block", s.adminBlock, adm)
	s.protected("POST /api/admin/users/{id}/unblock", s.adminUnblock, adm)
	s.protected("POST /api/admin/users/{id}/reset-password", s.adminResetPassword, adm)
}

func adminUserDTO(u model.User) dto.AdminUser {
	return dto.AdminUser{
		ID: u.ID, Login: u.Login, FullName: u.FullName, JobTitle: u.JobTitle,
		Department: deptRef(u.DepartmentID, u.DepartmentName), Role: u.Role, Status: u.Status, CreatedAt: u.CreatedAt,
	}
}

func adminErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, admin.ErrNotFound):
		mgrError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, admin.ErrLastAdmin):
		mgrError(w, http.StatusConflict, "last_admin")
	default:
		writeErr(w, http.StatusInternalServerError, "db error")
	}
}

// adminSetRole: admin only (route roles). Role is manager|employee. Self-change and
// demoting the last active admin are 409. All sessions of the target are revoked so a
// demoted manager loses powers immediately.
func (s *Server) adminSetRole(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	var in dto.RoleRequest
	if !mgrDecode(w, r, &in) {
		return
	}
	if in.Role != model.RoleManager && in.Role != model.RoleEmployee {
		mgrFieldErrors(w, dto.FieldError{Path: "role", Code: "invalid_role"})
		return
	}
	if id == auth.UserOf(r.Context()).UserID {
		mgrError(w, http.StatusConflict, "self_change")
		return
	}
	u, err := s.admin.ChangeRole(r.Context(), id, in.Role)
	if err != nil {
		adminErr(w, err)
		return
	}
	s.sessions.DestroyByUser(r.Context(), id)
	s.mgrAudit(r, "role_change", "user", id)
	writeJSON(w, http.StatusOK, adminUserDTO(u))
}

func (s *Server) adminBlock(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	if id == auth.UserOf(r.Context()).UserID {
		mgrError(w, http.StatusConflict, "self_block")
		return
	}
	u, err := s.admin.SetBlocked(r.Context(), id, true)
	if err != nil {
		adminErr(w, err)
		return
	}
	s.sessions.DestroyByUser(r.Context(), id)
	s.mgrAudit(r, "user_block", "user", id)
	writeJSON(w, http.StatusOK, adminUserDTO(u))
}

func (s *Server) adminUnblock(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	u, err := s.admin.SetBlocked(r.Context(), id, false)
	if err != nil {
		adminErr(w, err)
		return
	}
	s.mgrAudit(r, "user_unblock", "user", id)
	writeJSON(w, http.StatusOK, adminUserDTO(u))
}

// adminResetPassword: new hash + DestroyByUser (all sessions revoked). The password is
// never logged or audited.
func (s *Server) adminResetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	var in dto.PasswordRequest
	if !mgrDecode(w, r, &in) {
		return
	}
	if len(in.Password) < 8 || len(in.Password) > 128 {
		mgrFieldErrors(w, dto.FieldError{Path: "password", Code: "invalid_password"})
		return
	}
	if _, err := s.admin.Get(r.Context(), id); err != nil {
		adminErr(w, err)
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "hash error")
		return
	}
	if err := s.users.SetPassword(r.Context(), id, hash); err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	s.sessions.DestroyByUser(r.Context(), id)
	s.mgrAudit(r, "password_reset", "user", id)
	w.WriteHeader(http.StatusNoContent)
}
