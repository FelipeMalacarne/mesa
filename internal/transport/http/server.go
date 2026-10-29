// Package http serves the Mesa API and the embedded web app.
package http

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/transport/http/gen"
)

// Options configure the handler.
type Options struct {
	// Assets is the built web app (index.html at its root); nil serves no UI.
	Assets fs.FS
	// CORSOrigins allows browsers on other origins, for development.
	CORSOrigins []string
	Logger      *slog.Logger
	// MCP, when set, is mounted at /mcp so agents can reach the service over
	// HTTP with the same tools the stdio binary offers.
	MCP http.Handler
	// PublicURL is where people reach the server, for the sign-in callback
	// and secure cookies; empty reads it from each request.
	PublicURL string
}

// New builds the HTTP handler: /api/* from the contract, /healthz, and the
// single-page app for everything else.
func New(a *app.App, o Options) http.Handler {
	log := o.Logger
	if log == nil {
		log = slog.Default()
	}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(log))
	if len(o.CORSOrigins) > 0 {
		r.Use(cors(o.CORSOrigins))
	}
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	api := gen.NewStrictHandlerWithOptions(&handlers{app: a}, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  requestError,
		ResponseErrorHandlerFunc: responseError(log),
	})
	// Every API call and every MCP call runs as a principal: the signed-in
	// member, or the local one when this Mesa has no accounts.
	auth := authenticate(a, o)
	r.With(auth).Mount("/api", gen.HandlerWithOptions(api, gen.ChiServerOptions{
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, errs.Wrap(errs.Invalid, "bad request", err))
		},
	}))
	login, callback := signInRoutes(a, o)
	r.Get(loginPath, login)
	r.Get(callbackPath, callback)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, errs.New(errs.NotFound, "no such endpoint"))
			return
		}
		http.NotFound(w, r)
	})
	if o.MCP != nil {
		r.With(auth).Handle("/mcp", o.MCP)
		r.With(auth).Handle("/mcp/*", o.MCP)
	}
	if o.Assets != nil {
		r.Handle("/*", spa(o.Assets))
	}
	return r
}

// requestError handles malformed requests (bad JSON, bad params).
func requestError(w http.ResponseWriter, _ *http.Request, err error) {
	writeError(w, errs.Wrap(errs.Invalid, "bad request", err))
}

// responseError maps errors returned by handlers to the Error schema.
func responseError(log *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		if errs.KindOf(err) == errs.Unknown {
			log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		}
		writeError(w, err)
	}
}

func statusOf(err error) int {
	switch errs.KindOf(err) {
	case errs.NotFound:
		return http.StatusNotFound
	case errs.Invalid:
		return http.StatusBadRequest
	case errs.Forbidden:
		return http.StatusForbidden
	case errs.Conflict, errs.NeedsConfirmation:
		return http.StatusConflict
	case errs.Unauthenticated:
		return http.StatusUnauthorized
	case errs.Unavailable:
		return http.StatusBadGateway
	case errs.Unsupported:
		return http.StatusNotImplemented
	}
	return http.StatusInternalServerError
}

func writeError(w http.ResponseWriter, err error) {
	body := gen.Error{Kind: gen.ErrorKind(errs.KindOf(err).String()), Message: errs.Message(err)}
	if h := errs.Hint(err); h != "" {
		body.ConfirmWith = &h
	}
	if errs.KindOf(err) == errs.Unknown {
		body.Message = "something went wrong"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusOf(err))
	_ = json.NewEncoder(w).Encode(body)
}

func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Debug("http", "method", r.Method, "path", r.URL.Path, "status", ww.Status(), "ms", time.Since(start).Milliseconds())
		})
	}
}

func cors(origins []string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if o := r.Header.Get("Origin"); o != "" && (allowed["*"] || allowed[o]) {
				w.Header().Set("Access-Control-Allow-Origin", o)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Vary", "Origin")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// spa serves static files and falls back to index.html for client routes.
const notBuilt = `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Mesa</title></head>
<body style="font-family:system-ui,sans-serif;max-width:560px;margin:15vh auto;padding:0 16px;line-height:1.5">
<h1 style="font-size:20px">Mesa is running without its web app</h1>
<p>This binary was built before the web app. Run <code>make web</code>, then rebuild <code>mesa-server</code>. The API is available under <code>/api</code>.</p>
</body></html>`

func spa(assets fs.FS) http.Handler {
	files := http.FS(assets)
	fileServer := http.FileServer(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean("/" + r.URL.Path)
		if p != "/" {
			if f, err := assets.Open(strings.TrimPrefix(p, "/")); err == nil {
				_ = f.Close()
				if strings.HasPrefix(p, "/assets/") || strings.HasPrefix(p, "/fonts/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(w, r)
				return
			} else if !errors.Is(err, fs.ErrNotExist) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		index, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			// Built without the web app (`make web` was not run): say so.
			index = []byte(notBuilt)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}
