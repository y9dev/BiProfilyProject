package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/invitations"
)

const (
	maxRegistrationsLimit = 10000
	registrantsDefault    = 100
	registrantsMax        = 500
)

// invitationDTO maps a row to the frozen shape; creators caches id -> full name.
func (s *Server) invitationDTO(r *http.Request, inv invitations.Invitation, creators map[int64]string) dto.Invitation {
	name, ok := creators[inv.CreatedBy]
	if !ok {
		if u, err := s.users.GetByID(r.Context(), inv.CreatedBy); err == nil {
			name = u.FullName
		}
		creators[inv.CreatedBy] = name
	}
	return dto.Invitation{
		ID: inv.ID, Code: inv.Code, URL: s.origin + "/register?code=" + inv.Code,
		ExpiresAt: inv.ExpiresAt, MaxRegistrations: inv.MaxRegistrations, RegistrationsCount: inv.RegistrationsCount,
		Status: invitations.Status(inv, time.Now()), CreatedAt: inv.CreatedAt,
		CreatedBy: dto.UserRef{ID: inv.CreatedBy, FullName: name},
	}
}

func (s *Server) invitationsList(w http.ResponseWriter, r *http.Request) {
	list, err := s.invRepo().List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	creators := map[int64]string{}
	out := make([]dto.Invitation, 0, len(list))
	for _, inv := range list {
		out = append(out, s.invitationDTO(r, inv, creators))
	}
	writeJSON(w, http.StatusOK, out)
}

func validMax(n int) bool { return n >= 1 && n <= maxRegistrationsLimit }

func (s *Server) invitationsCreate(w http.ResponseWriter, r *http.Request) {
	var in dto.InvitationCreateRequest
	if !mgrDecode(w, r, &in) {
		return
	}
	var errs []dto.FieldError
	if !validMax(in.MaxRegistrations) {
		errs = append(errs, dto.FieldError{Path: "maxRegistrations", Code: "out_of_range"})
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
		errs = append(errs, dto.FieldError{Path: "expiresAt", Code: "past_expiry"})
	}
	if len(errs) > 0 {
		mgrFieldErrors(w, errs...)
		return
	}
	sess := auth.UserOf(r.Context())
	inv, err := s.invRepo().Create(r.Context(), invitations.CreateInput{
		CreatedBy: sess.UserID, ExpiresAt: in.ExpiresAt, MaxRegistrations: in.MaxRegistrations,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	s.mgrAudit(r, "invitation_create", "invitation", inv.ID)
	writeJSON(w, http.StatusCreated, s.invitationDTO(r, inv, map[int64]string{}))
}

func (s *Server) invitationsPatch(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	var raw map[string]json.RawMessage
	if !mgrDecode(w, r, &raw) {
		return
	}
	var p invitations.Patch
	var errs []dto.FieldError
	if v, present := raw["expiresAt"]; present {
		p.SetExpiry = true
		if err := json.Unmarshal(v, &p.ExpiresAt); err != nil {
			errs = append(errs, dto.FieldError{Path: "expiresAt", Code: "invalid"})
		} else if p.ExpiresAt != nil && !p.ExpiresAt.After(time.Now()) {
			errs = append(errs, dto.FieldError{Path: "expiresAt", Code: "past_expiry"})
		}
	}
	if v, present := raw["maxRegistrations"]; present {
		var n int
		if err := json.Unmarshal(v, &n); err != nil || !validMax(n) {
			errs = append(errs, dto.FieldError{Path: "maxRegistrations", Code: "out_of_range"})
		} else {
			p.MaxRegistrations = &n
		}
	}
	if len(errs) > 0 {
		mgrFieldErrors(w, errs...)
		return
	}
	inv, err := s.invRepo().Update(r.Context(), id, p)
	switch {
	case errors.Is(err, invitations.ErrNotFound):
		mgrError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, invitations.ErrMaxBelowCount):
		mgrError(w, http.StatusConflict, "max_below_count")
	case errors.Is(err, invitations.ErrRevoked):
		mgrError(w, http.StatusConflict, "revoked")
	case errors.Is(err, invitations.ErrPastExpiry):
		mgrFieldErrors(w, dto.FieldError{Path: "expiresAt", Code: "past_expiry"})
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "db error")
	default:
		s.mgrAudit(r, "invitation_update", "invitation", id)
		writeJSON(w, http.StatusOK, s.invitationDTO(r, inv, map[int64]string{}))
	}
}

func (s *Server) invitationsRevoke(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	repo := s.invRepo()
	if err := repo.Revoke(r.Context(), id); err != nil {
		if errors.Is(err, invitations.ErrNotFound) {
			mgrError(w, http.StatusNotFound, "not_found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	inv, err := repo.Get(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	s.mgrAudit(r, "invitation_revoke", "invitation", id)
	writeJSON(w, http.StatusOK, s.invitationDTO(r, inv, map[int64]string{}))
}

func (s *Server) invitationsDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	err := s.invRepo().Delete(r.Context(), id)
	switch {
	case errors.Is(err, invitations.ErrNotFound):
		mgrError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, invitations.ErrHasRegistrations):
		mgrError(w, http.StatusConflict, "has_registrations")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "db error")
	default:
		s.mgrAudit(r, "invitation_delete", "invitation", id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// invitationsRegistrations: paginated (?limit default 100, max 500; ?offset); the
// total is in X-Total-Count (the registrations_count of the invitation).
func (s *Server) invitationsRegistrations(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = registrantsDefault
	}
	limit = min(limit, registrantsMax)
	offset = max(offset, 0)
	repo := s.invRepo()
	inv, err := repo.Get(r.Context(), id)
	if errors.Is(err, invitations.ErrNotFound) {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	list, err := repo.RegistrantsPage(r.Context(), id, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	out := make([]dto.Registrant, 0, len(list))
	for _, g := range list {
		out = append(out, dto.Registrant{FullName: g.FullName, JobTitle: g.JobTitle, Department: deptRef(g.DepartmentID, g.DepartmentName)})
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(inv.RegistrationsCount))
	writeJSON(w, http.StatusOK, out)
}
