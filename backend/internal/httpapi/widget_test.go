package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"bigroup-backend/internal/auth"
	"bigroup-backend/internal/httpapi/dto"
	"bigroup-backend/internal/model"
	"bigroup-backend/internal/users"
)

type fakeUsers struct {
	users.Repository
	byID map[int64]model.User
}

func (f fakeUsers) GetByID(_ context.Context, id int64) (model.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return model.User{}, pgx.ErrNoRows
}

func widgetServer(us map[int64]model.User) *Server {
	s := New(Deps{Users: fakeUsers{byID: us}, Sessions: auth.NewManager(nil, false)})
	s.widgetProfile = func(context.Context, int64) (dto.Profile, error) {
		i18n := map[string]string{"ru": "Коммуникация", "kk": "Коммуникация-kk"}
		return dto.Profile{Characteristics: []dto.ProfileCharacteristic{
			{ID: 1, Name: "c1", NameI18n: i18n, Mean: 80}, {ID: 2, Name: "c2", NameI18n: i18n, Mean: 60},
			{ID: 3, Name: "c3", NameI18n: i18n, Mean: 40}, {ID: 4, Name: "c4", NameI18n: i18n, Mean: 20},
			{ID: 9, Name: "extra", Mean: 70},
		}}, nil
	}
	return s
}

func getWidget(s *Server, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestWidgetHandler(t *testing.T) {
	dept := "Отдел <b>продаж</b>"
	s := widgetServer(map[int64]model.User{
		7: {ID: 7, FullName: `<script>alert(1)</script>`, JobTitle: "Dev", DepartmentName: &dept, Status: model.UserActive},
		8: {ID: 8, FullName: "Blocked", Status: model.UserBlocked},
	})

	if rec := getWidget(s, "/api/widget/abc"); rec.Code != 400 {
		t.Errorf("non-numeric: %d", rec.Code)
	}
	if rec := getWidget(s, "/api/widget/0"); rec.Code != 400 {
		t.Errorf("zero: %d", rec.Code)
	}
	if rec := getWidget(s, "/api/widget/99"); rec.Code != 404 {
		t.Errorf("unknown: %d", rec.Code)
	}
	if rec := getWidget(s, "/api/widget/8"); rec.Code != 404 {
		t.Errorf("blocked: %d", rec.Code)
	}

	rec := getWidget(s, "/api/widget/7?lang=kk")
	if rec.Code != 200 {
		t.Fatalf("ok: %d %s", rec.Code, rec.Body)
	}
	h := rec.Header()
	if !strings.HasPrefix(h.Get("Content-Type"), "text/html") || h.Get("Cache-Control") != "no-cache" ||
		h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Content-Security-Policy") != widgetCSP {
		t.Errorf("headers: %v", h)
	}
	if h.Get("X-Frame-Options") != "" {
		t.Errorf("X-Frame-Options must be absent, got %q", h.Get("X-Frame-Options"))
	}
	body := rec.Body.String()
	if strings.Contains(body, "<script") || strings.Contains(body, "<b>") {
		t.Errorf("unescaped user text")
	}
	if !strings.Contains(body, "Коммуникация-kk") || !strings.Contains(body, "Дағдылар профилі") {
		t.Errorf("kk labels missing")
	}
}

func TestWidgetRouteIsPublicAndGET(t *testing.T) {
	for _, r := range offlineServer().Routes() {
		if r.Path == "/api/widget/{userId}" {
			if !r.Public || r.Method != "GET" {
				t.Errorf("widget route must be public GET: %+v", r)
			}
			return
		}
	}
	t.Fatal("widget route not registered")
}

func TestWidgetAccessSeamAllowsToday(t *testing.T) {
	if err := offlineServer().widgetAccess(httptest.NewRequest("GET", "/", nil), 1); err != nil {
		t.Fatalf("widgetAccess must allow today: %v", err)
	}
	_ = http.StatusOK
}
