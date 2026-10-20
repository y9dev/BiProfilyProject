package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"bigroup-backend/internal/assessments"
	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
)

const (
	maxCreateBody = 1 << 20 // 1 MiB

	idemHeader      = "Idempotency-Key"
	idemPendingTTL  = 60 * time.Second
	idemDoneTTL     = 15 * time.Minute
	idemPendingMark = "pending"
)

// idemKey is the Redis key for a user's create-assessment idempotency key ("" = none given).
func idemKey(userID int64, key string) string {
	if key == "" || len(key) > 100 {
		return ""
	}
	return "idem:assessment_create:" + itoa(userID) + ":" + key
}

// registerAssessmentCreateRoutes: POST /api/manage/assessments (manager and admin).
func (s *Server) registerAssessmentCreateRoutes() {
	s.protected("POST /api/manage/assessments", s.createAssessment, routeOpts{roles: rolesManager})
}

func (s *Server) createAssessment(w http.ResponseWriter, r *http.Request) {
	sess := auth.UserOf(r.Context())
	var req dto.CreateAssessmentRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCreateBody)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, dto.Error{Error: "bad_request"})
		return
	}
	// Idempotency: a retry or double click carrying the same key must not create a second
	// assessment. The first request claims the key; a concurrent duplicate gets 409, and a
	// duplicate after completion replays the stored result.
	ik := idemKey(sess.UserID, r.Header.Get(idemHeader))
	if ik != "" && s.rdb != nil {
		claimed, err := s.rdb.SetNX(r.Context(), ik, idemPendingMark, idemPendingTTL).Result()
		if err == nil && !claimed {
			val, gerr := s.rdb.Get(r.Context(), ik).Result()
			if gerr == nil && val != idemPendingMark {
				var prev dto.CreateAssessmentResult
				if json.Unmarshal([]byte(val), &prev) == nil {
					writeJSON(w, http.StatusCreated, prev)
					return
				}
			}
			writeJSON(w, http.StatusConflict, dto.Error{Error: "in_progress"})
			return
		}
		if err != nil {
			ik = "" // Redis trouble: fall through without idempotency rather than fail the request
		}
	}
	release := func() {
		if ik != "" && s.rdb != nil {
			s.rdb.Del(context.Background(), ik)
		}
	}
	creator := assessments.NewCreator(s.pool)
	known, err := creator.KnownCharacteristics(r.Context())
	if err != nil {
		release()
		writeErr(w, http.StatusInternalServerError, "internal")
		return
	}
	errs, warns := assessments.Validate(req, known, time.Now())
	if len(errs) > 0 {
		release()
		writeJSON(w, http.StatusUnprocessableEntity, dto.Error{Error: "validation", Errors: errs})
		return
	}
	id, err := creator.Create(r.Context(), sess.UserID, req)
	if err != nil {
		release()
		writeErr(w, http.StatusInternalServerError, "internal")
		return
	}
	if warns == nil {
		warns = []dto.Warning{}
	}
	s.audit(r, model.AuditEntry{ActorID: &sess.UserID, ActorRole: sess.Role, Action: "assessment_create",
		EntityType: "assessment", EntityID: itoa(id), Result: "ok"})
	result := dto.CreateAssessmentResult{ID: id, Warnings: warns}
	if ik != "" && s.rdb != nil {
		if b, merr := json.Marshal(result); merr == nil {
			s.rdb.Set(context.Background(), ik, string(b), idemDoneTTL)
		}
	}
	writeJSON(w, http.StatusCreated, result)
}
