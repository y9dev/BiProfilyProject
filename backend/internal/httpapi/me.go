package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/avatar"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
)

// avatarCatalog: GET /api/avatar/catalog (avatar-exempt).
func (s *Server) avatarCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, dto.AvatarCatalog{Universes: avatar.Universes, Genders: avatar.Genders, Styles: avatar.Styles})
}

// setMyAvatar: PUT /api/me/avatar (avatar-exempt). Completes the profile and flips
// ProfileComplete in every live session of the user, not only the calling one.
func (s *Server) setMyAvatar(w http.ResponseWriter, r *http.Request) {
	sess := auth.UserOf(r.Context())
	var in dto.AvatarRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, dto.Error{Error: "bad json"})
		return
	}
	a := model.Avatar{Universe: in.Universe, Gender: in.Gender, Style: in.Style}
	if !avatar.Valid(a) {
		writeJSON(w, http.StatusUnprocessableEntity, dto.Error{Error: "validation",
			Errors: []dto.FieldError{{Path: "avatar", Code: "not_in_catalog"}}})
		return
	}
	if err := s.users.SetAvatar(r.Context(), sess.UserID, a); err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	s.sessions.UpdateProfileCompleteForUser(r.Context(), sess.UserID, true)
	u, err := s.users.GetByID(r.Context(), sess.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusUnauthorized, dto.Error{Error: "unauthorized"})
			return
		}
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	s.audit(r, model.AuditEntry{ActorID: &sess.UserID, ActorRole: sess.Role, Action: "avatar_set", EntityType: "user", EntityID: itoa(sess.UserID), Result: "ok"})
	writeJSON(w, http.StatusOK, meOf(u))
}
