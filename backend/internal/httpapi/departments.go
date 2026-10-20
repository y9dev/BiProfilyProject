package httpapi

import (
	"errors"
	"net/http"

	"bigroup-backend/internal/departments"
	"bigroup-backend/internal/httpapi/dto"
)

const maxDeptName = 120

func deptDTO(it departments.Item) dto.Department {
	return dto.Department{ID: it.ID, Name: it.Name, Active: it.Active, UserCount: it.UserCount}
}

func (s *Server) departmentsList(w http.ResponseWriter, r *http.Request) {
	items, err := s.deptRepo().List(r.Context(), r.URL.Query().Get("includeInactive") == "1")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	out := make([]dto.Department, 0, len(items))
	for _, it := range items {
		out = append(out, deptDTO(it))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) departmentWrite(w http.ResponseWriter, r *http.Request, create bool) {
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
	name, ferr := mgrName(in.Name, maxDeptName)
	if ferr != nil {
		mgrFieldErrors(w, *ferr)
		return
	}
	var it departments.Item
	var err error
	if create {
		it, err = s.deptRepo().Create(r.Context(), name)
	} else {
		it, err = s.deptRepo().Rename(r.Context(), id, name)
	}
	switch {
	case errors.Is(err, departments.ErrDuplicate):
		mgrError(w, http.StatusConflict, "duplicate_name")
	case errors.Is(err, departments.ErrNotFound):
		mgrError(w, http.StatusNotFound, "not_found")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "db error")
	case create:
		s.mgrAudit(r, "department_create", "department", it.ID)
		writeJSON(w, http.StatusCreated, deptDTO(it))
	default:
		s.mgrAudit(r, "department_rename", "department", id)
		writeJSON(w, http.StatusOK, deptDTO(it))
	}
}

func (s *Server) departmentsCreate(w http.ResponseWriter, r *http.Request) {
	s.departmentWrite(w, r, true)
}
func (s *Server) departmentsRename(w http.ResponseWriter, r *http.Request) {
	s.departmentWrite(w, r, false)
}

// departmentsDelete: 204 when an unused department was removed, 200 {deactivated:true}
// when it is in use (users keep it).
func (s *Server) departmentsDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	deactivated, err := s.deptRepo().Delete(r.Context(), id)
	switch {
	case errors.Is(err, departments.ErrNotFound):
		mgrError(w, http.StatusNotFound, "not_found")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "db error")
	case deactivated:
		s.mgrAudit(r, "department_deactivate", "department", id)
		writeJSON(w, http.StatusOK, dto.DepartmentDeleteResult{Deactivated: true})
	default:
		s.mgrAudit(r, "department_delete", "department", id)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) departmentsActivate(w http.ResponseWriter, r *http.Request) {
	id, ok := mgrID(r)
	if !ok {
		mgrError(w, http.StatusNotFound, "not_found")
		return
	}
	it, err := s.deptRepo().Activate(r.Context(), id)
	switch {
	case errors.Is(err, departments.ErrDuplicate):
		mgrError(w, http.StatusConflict, "duplicate_name")
	case errors.Is(err, departments.ErrNotFound):
		mgrError(w, http.StatusNotFound, "not_found")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "db error")
	default:
		s.mgrAudit(r, "department_activate", "department", id)
		writeJSON(w, http.StatusOK, deptDTO(it))
	}
}
