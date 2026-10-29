package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

// zoomLevels are the stops View → Zoom In and Zoom Out step through, the
// same ones a browser uses.
var zoomLevels = []float64{0.5, 0.67, 0.75, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2, 2.5, 3}

// Zoom is the desktop window's zoom level. WKWebView and WebKitGTK have no
// zoom of their own, so the app keeps the level and saves it in a file: the
// next launch opens at the same size.
type Zoom struct {
	mu    sync.Mutex
	path  string
	level float64
}

// NewZoom reads the level saved at path; a missing or unreadable file is
// 100%.
func NewZoom(path string) *Zoom {
	z := &Zoom{path: path, level: 1}
	if b, err := os.ReadFile(path); err == nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64); err == nil && v >= zoomLevels[0] && v <= zoomLevels[len(zoomLevels)-1] {
			z.level = v
		}
	}
	return z
}

// Level is the current zoom level, 1 for 100%.
func (z *Zoom) Level() float64 {
	z.mu.Lock()
	defer z.mu.Unlock()
	return z.level
}

// In steps to the next level up and saves it.
func (z *Zoom) In() (float64, error) {
	return z.set(func(cur float64) float64 {
		for _, l := range zoomLevels {
			if l > cur+0.001 {
				return l
			}
		}
		return cur
	})
}

// Out steps to the next level down and saves it.
func (z *Zoom) Out() (float64, error) {
	return z.set(func(cur float64) float64 {
		for i := len(zoomLevels) - 1; i >= 0; i-- {
			if zoomLevels[i] < cur-0.001 {
				return zoomLevels[i]
			}
		}
		return cur
	})
}

// Reset goes back to 100% and saves it.
func (z *Zoom) Reset() (float64, error) {
	return z.set(func(float64) float64 { return 1 })
}

func (z *Zoom) set(next func(float64) float64) (float64, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.level = next(z.level)
	return z.level, os.WriteFile(z.path, []byte(strconv.FormatFloat(z.level, 'f', -1, 64)+"\n"), 0o600)
}

// Handler serves the zoom to the web app's Settings page: GET answers
// {"level":1.25,"native":true}; POST {"step":"in"|"out"|"reset"} changes it,
// calls apply with the new level and answers the same shape. native says
// whether the app zooms the window itself (false where the webview has its
// own zoom keys).
func (z *Zoom) Handler(native bool, apply func(level float64)) http.Handler {
	type state struct {
		Level  float64 `json:"level"`
		Native bool    `json:"native"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		level := z.Level()
		switch r.Method {
		case http.MethodGet:
		case http.MethodPost:
			if !native {
				http.Error(w, "this window zooms with its own keys", http.StatusConflict)
				return
			}
			var body struct {
				Step string `json:"step"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
				http.Error(w, "body must be {\"step\": \"in\" | \"out\" | \"reset\"}", http.StatusBadRequest)
				return
			}
			var err error
			switch body.Step {
			case "in":
				level, err = z.In()
			case "out":
				level, err = z.Out()
			case "reset":
				level, err = z.Reset()
			default:
				http.Error(w, "step must be in, out or reset", http.StatusBadRequest)
				return
			}
			apply(level)
			if err != nil {
				http.Error(w, "the zoom changed but could not be saved: "+err.Error(), http.StatusInternalServerError)
				return
			}
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(state{Level: level, Native: native})
	})
}
