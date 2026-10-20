package httpapi

// GET /api/widget/{userId}: public, embeddable HTML card (see internal/widget).

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"bigroup-backend/internal/attempts"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
	"bigroup-backend/internal/widget"
)

// widgetCSP allows only the inline <style> and data: images the fragment uses.
const widgetCSP = "default-src 'none'; style-src 'unsafe-inline'; img-src data:"

// widgetAccess is the authorization seam for the widget.
//
// ACCEPTED RISK (IDOR): today it returns nil, so anyone who can reach the API can
// render the card of any user id (enumerable numeric ids). Exposed data is limited to
// full name, job title, department, avatar and per-characteristic means, by product
// decision (the card is meant to be embedded elsewhere). To restrict it later, return
// an error here (e.g. check a signed token or the session in r) and the handler
// answers 403.
func (s *Server) widgetAccess(r *http.Request, userID int64) error { return nil }

func (s *Server) registerWidgetRoutes() {
	s.widgetProfile = func(ctx context.Context, id int64) (dto.Profile, error) {
		return attempts.New(s.pool).Profile(ctx, id)
	}
	s.public("GET /api/widget/{userId}", s.widgetLim.Limit(http.HandlerFunc(s.widget)))
}

func (s *Server) widget(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("userId"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "bad_user_id")
		return
	}
	if err := s.widgetAccess(r, id); err != nil {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	u, err := s.users.GetByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && u.Status == model.UserBlocked) {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		log.Printf("widget: user: %v", err)
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	p, err := s.widgetProfile(r.Context(), id)
	if err != nil {
		log.Printf("widget: profile: %v", err)
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	html, err := widget.Render(widget.FromProfile(u, p, r.URL.Query().Get("lang")))
	if err != nil {
		log.Printf("widget: render: %v", err)
		writeErr(w, http.StatusInternalServerError, "render error")
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", widgetCSP)
	h.Del("X-Frame-Options") // embeddable by design; securityHeaders sets DENY globally
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}
