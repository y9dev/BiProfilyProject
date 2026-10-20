package httpapi

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLogRequests_MarksClientCanceled: an aborted request (canceled context) whose handler
// fails with 500 is logged as "client canceled", so it is not read as a server fault.
func TestLogRequests_MarksClientCanceled(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	h := logRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Err() != nil {
			writeErr(w, http.StatusInternalServerError, "db error")
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/auth/me", nil).WithContext(ctx))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 log lines, got %q", buf.String())
	}
	if strings.Contains(lines[0], "canceled") || !strings.Contains(lines[0], "-> 200") {
		t.Errorf("normal request line: %q", lines[0])
	}
	if !strings.Contains(lines[1], "-> 500") || !strings.HasSuffix(lines[1], "client canceled") {
		t.Errorf("canceled request line: %q", lines[1])
	}
}
