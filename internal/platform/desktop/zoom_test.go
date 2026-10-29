package desktop

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZoomSteps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zoom")
	z := NewZoom(path)
	if got := z.Level(); got != 1 {
		t.Fatalf("new zoom = %v, want 1", got)
	}
	for _, want := range []float64{1.1, 1.25, 1.5} {
		if got, err := z.In(); err != nil || got != want {
			t.Fatalf("In() = %v, %v; want %v", got, err, want)
		}
	}
	if got, _ := z.Out(); got != 1.25 {
		t.Fatalf("Out() = %v, want 1.25", got)
	}
	// The level survives a restart.
	if got := NewZoom(path).Level(); got != 1.25 {
		t.Fatalf("reloaded zoom = %v, want 1.25", got)
	}
	if got, _ := z.Reset(); got != 1 {
		t.Fatalf("Reset() = %v, want 1", got)
	}
}

func TestZoomBounds(t *testing.T) {
	z := NewZoom(filepath.Join(t.TempDir(), "zoom"))
	for range 20 {
		_, _ = z.In()
	}
	if got := z.Level(); got != 3 {
		t.Fatalf("zoom after many In() = %v, want 3", got)
	}
	for range 30 {
		_, _ = z.Out()
	}
	if got := z.Level(); got != 0.5 {
		t.Fatalf("zoom after many Out() = %v, want 0.5", got)
	}
}

func TestZoomIgnoresBadFile(t *testing.T) {
	for _, content := range []string{"", "abc", "0", "40", "-1"} {
		path := filepath.Join(t.TempDir(), "zoom")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := NewZoom(path).Level(); got != 1 {
			t.Errorf("NewZoom with %q = %v, want 1", content, got)
		}
	}
	// A level saved between stops (an older build's) steps to its neighbours.
	path := filepath.Join(t.TempDir(), "zoom")
	if err := os.WriteFile(path, []byte("1.2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	z := NewZoom(path)
	if got, _ := z.In(); got != 1.25 {
		t.Errorf("In() from 1.2 = %v, want 1.25", got)
	}
}

func TestZoomHandler(t *testing.T) {
	z := NewZoom(filepath.Join(t.TempDir(), "zoom"))
	var applied []float64
	h := z.Handler(true, func(level float64) { applied = append(applied, level) })
	do := func(method, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/desktop/zoom", strings.NewReader(body)))
		return rec
	}

	if rec := do(http.MethodGet, ""); rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"level":1,"native":true}` {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	if rec := do(http.MethodPost, `{"step":"in"}`); rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"level":1.1,"native":true}` {
		t.Fatalf("POST in = %d %s", rec.Code, rec.Body)
	}
	if rec := do(http.MethodPost, `{"step":"sideways"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("POST sideways = %d, want 400", rec.Code)
	}
	if rec := do(http.MethodDelete, ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE = %d, want 405", rec.Code)
	}
	if len(applied) != 1 || applied[0] != 1.1 {
		t.Fatalf("applied = %v, want [1.1]", applied)
	}

	// Where the webview zooms by itself the level is only reported.
	rec := httptest.NewRecorder()
	z.Handler(false, func(float64) { t.Fatal("apply called") }).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"step":"in"}`)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("POST without native zoom = %d, want 409", rec.Code)
	}
}
