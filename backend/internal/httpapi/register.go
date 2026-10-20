package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/departments"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/invitations"
	"bigroup-backend/internal/model"
	"bigroup-backend/internal/users"
)

var loginRe = regexp.MustCompile(`^[A-Za-z0-9._-]{3,64}$`)

func validateRegister(in dto.RegisterRequest, code string) []dto.FieldError {
	var errs []dto.FieldError
	add := func(path, c string) { errs = append(errs, dto.FieldError{Path: path, Code: c}) }
	if code == "" {
		add("code", "required")
	}
	if in.Login == "" {
		add("login", "required")
	} else if !loginRe.MatchString(in.Login) {
		add("login", "invalid")
	}
	switch n := utf8.RuneCountInString(in.Password); {
	case n == 0:
		add("password", "required")
	case n < 8:
		add("password", "too_short")
	case n > 128:
		add("password", "too_long")
	}
	switch n := utf8.RuneCountInString(in.FullName); {
	case n == 0:
		add("fullName", "required")
	case n > 160:
		add("fullName", "too_long")
	}
	if utf8.RuneCountInString(in.JobTitle) > 160 {
		add("jobTitle", "too_long")
	}
	return errs
}

// register: POST /api/auth/register. Field validation (422), then ONE transaction:
// atomic invitation redemption -> user insert. A taken login rolls the transaction
// back, so the invitation count is restored. Invalid invitation reasons are one
// generic 410 invite_invalid.
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var in dto.RegisterRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, dto.Error{Error: "bad json"})
		return
	}
	code := invitations.NormalizeCode(in.Code)
	in.Login = strings.TrimSpace(in.Login)
	in.FullName = strings.TrimSpace(in.FullName)
	in.JobTitle = strings.TrimSpace(in.JobTitle)
	if errs := validateRegister(in, code); len(errs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, dto.Error{Error: "validation", Errors: errs})
		return
	}
	ctx := r.Context()
	if in.DepartmentID != nil {
		list, err := departments.New(s.pool).ListActive(ctx)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "db error")
			return
		}
		ok := slices.ContainsFunc(list, func(d model.Department) bool { return d.ID == *in.DepartmentID })
		if !ok {
			writeJSON(w, http.StatusUnprocessableEntity, dto.Error{Error: "validation",
				Errors: []dto.FieldError{{Path: "departmentId", Code: "invalid"}}})
			return
		}
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "hash error")
		return
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit

	invID, err := invitations.New(s.pool).Redeem(ctx, tx, code)
	if err != nil {
		if errors.Is(err, invitations.ErrInviteInvalid) {
			_ = tx.Rollback(ctx) // release the conn before audit needs another one
			s.audit(r, model.AuditEntry{Action: "register_deny", EntityType: "invitation", Result: "invite_invalid"})
			writeJSON(w, http.StatusGone, dto.Error{Error: "invite_invalid"})
			return
		}
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	userID, err := s.users.CreateTx(ctx, tx, model.User{
		Login: in.Login, FullName: in.FullName, JobTitle: in.JobTitle,
		DepartmentID: in.DepartmentID, InvitationID: &invID, Role: model.RoleEmployee,
	}, hash)
	if err != nil {
		if errors.Is(err, users.ErrLoginTaken) {
			_ = tx.Rollback(ctx) // restores the invitation count
			s.audit(r, model.AuditEntry{Action: "register_deny", EntityType: "invitation", EntityID: itoa(invID), Result: "login_taken"})
			writeJSON(w, http.StatusConflict, dto.Error{Error: "login_taken"})
			return
		}
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}

	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	s.sessions.Destroy(ctx, auth.SessionTokenOf(r))
	token, err := s.sessions.Create(ctx, u.ID, u.Role, u.ProfileComplete())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	csrf, err := newCsrf()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	s.sessions.SetSessionCookie(w, token)
	s.sessions.SetCsrfCookie(w, csrf)
	s.audit(r, model.AuditEntry{ActorID: &u.ID, ActorRole: u.Role, Action: "register_ok", EntityType: "invitation", EntityID: itoa(invID), Result: "ok"})
	writeJSON(w, http.StatusCreated, meOf(u))
}
