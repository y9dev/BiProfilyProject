package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"bigroup-backend/internal/characteristics"
	"bigroup-backend/internal/httpapi/dto"
)

const maxCharName = 120

func charDTO(it characteristics.Item) dto.Characteristic {
	return dto.Characteristic{ID: it.ID, Name: it.Name, NameI18n: it.NameI18n, IsDefault: it.IsDefault, InUse: it.InUse}
}

func (s *Server) characteristicsList(w http.ResponseWriter, r *http.Request) {
	items, err := s.charRepo().List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	out := make([]dto.Characteristic, 0, len(items))
	for _, it := range items {
		out = append(out, charDTO(it))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) characteristicWrite(w http.ResponseWriter, r *http.Request, create bool) {
	var id int64
	if !create {
		var ok bool
		if id, ok = mgrID(r); !ok {
			mgrError(w, http.StatusNotFound, "not_found")
			return
		}
	}
	var in dto.NameRequest
	if !mgrDecode(w, r, &in) {
		return
	}
	name, ferr := mgrName(in.Name, maxCharName)
	if ferr != nil {
		mgrFieldErrors(w, *ferr)
		return
	}
	var it characteristics.Item
	var err error
	if create {
		it, err = s.charRepo().Create(r.Context(), name)
	} else {
		it, err = s.charRepo().Rename(r.Context(), id, name)
	}
	switch {
	case errors.Is(err, characteristics.ErrDuplicate):
		mgrError(w, http.StatusConflict, "duplicate_name")
	case errors.Is(err, characteristics.ErrDefaultProtected):
		mgrError(w, http.StatusConflict, "default_protected")
	case errors.Is(err, characteristics.ErrNotFound):
		mgrError(w, http.StatusNotFound, "not_found")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "db error")
	case create:
		s.mgrAudit(r, "characteristic_create", "characteristic", it.ID)
		writeJSON(w, http.StatusCreated, charDTO(it))
	default:
		s.mgrAudit(r, "characteristic_rename", "characteristic", id)
		writeJSON(w, http.StatusOK, charDTO(it))
	}
}

func (s *Server) characteristicsCreate(w http.ResponseWriter, r *http.Request) {
	s.characteristicWrite(w, r, true)
}
func (s *Server) characteristicsRename(w http.ResponseWriter, r *http.Request) {
	s.characteristicWrite(w, r, false)
}

// characteristicsDelete: 409 default_protected / in_use. For in_use the body lists the
// assessments that use it: errors[] = {path:"assessments[<id>]", code:"in_use"} and
// message = their titles (no contract change, existing dto.Error fields).
func (s *Server) characteristicsDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	err := s.charRepo().Delete(r.Context(), id)
	var inUse *characteristics.InUseError
	switch {
	case errors.As(err, &inUse):
		body := dto.Error{Error: "in_use"}
		titles := make([]string, 0, len(inUse.Assessments))
		for _, a := range inUse.Assessments {
			body.Errors = append(body.Errors, dto.FieldError{Path: fmt.Sprintf("assessments[%d]", a.ID), Code: "in_use"})
			titles = append(titles, a.Title)
		}
		body.Message = strings.Join(titles, "; ")
		writeJSON(w, http.StatusConflict, body)
	case errors.Is(err, characteristics.ErrDefaultProtected):
		mgrError(w, http.StatusConflict, "default_protected")
	case errors.Is(err, characteristics.ErrNotFound):
		mgrError(w, http.StatusNotFound, "not_found")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "db error")
	default:
		s.mgrAudit(r, "characteristic_delete", "characteristic", id)
		w.WriteHeader(http.StatusNoContent)
	}
}
