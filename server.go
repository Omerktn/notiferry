package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

const deliveryTimeout = 10 * time.Second

type configStore struct{ value atomic.Value }

func (s *configStore) Load() Config   { return s.value.Load().(Config) }
func (s *configStore) Store(c Config) { s.value.Store(c) }

type App struct {
	config *configStore
	sender Sender
	logger *slog.Logger
}

type notifyRequest struct {
	Text   string  `json:"text"`
	Target *string `json:"target"`
	Format string  `json:"format"`
}
type receipt struct {
	RequestID string `json:"request_id"`
	Target    string `json:"target"`
	Provider  string `json:"provider"`
	Chunks    int    `json:"chunks"`
	Delivered int    `json:"delivered"`
}
type apiError struct {
	RequestID string   `json:"request_id"`
	Error     string   `json:"error"`
	Message   string   `json:"message"`
	Receipt   *receipt `json:"receipt,omitempty"`
}

func NewApp(c Config, sender Sender, logger *slog.Logger) *App {
	s := &configStore{}
	s.Store(c)
	if logger == nil {
		logger = slog.Default()
	}
	return &App{config: s, sender: sender, logger: logger}
}
func (a *App) Reload(c Config) { a.config.Store(c) }
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", a.live)
	mux.HandleFunc("GET /health/ready", a.ready)
	mux.HandleFunc("POST /v1/notify", a.notify)
	return mux
}
func jsonWrite(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (a *App) live(w http.ResponseWriter, _ *http.Request) {
	jsonWrite(w, 200, map[string]string{"status": "ok"})
}
func (a *App) ready(w http.ResponseWriter, _ *http.Request) {
	jsonWrite(w, 200, map[string]string{"status": "ready"})
}
func requestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
func (a *App) notify(w http.ResponseWriter, r *http.Request) {
	id, started := requestID(), time.Now()
	var req notifyRequest
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			jsonWrite(w, http.StatusRequestEntityTooLarge, apiError{id, "request_too_large", "request body exceeds the limit", nil})
			return
		}
		jsonWrite(w, 400, apiError{id, "invalid_request", "request must be valid JSON", nil})
		return
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			jsonWrite(w, http.StatusRequestEntityTooLarge, apiError{id, "request_too_large", "request body exceeds the limit", nil})
			return
		}
		jsonWrite(w, 400, apiError{id, "invalid_request", "request must contain one JSON object", nil})
		return
	}
	if req.Text == "" {
		jsonWrite(w, 400, apiError{id, "invalid_text", "text is required", nil})
		return
	}
	if len([]byte(req.Text)) > 64*1024 {
		jsonWrite(w, 413, apiError{id, "text_too_large", "text exceeds 64 KiB", nil})
		return
	}
	if req.Format == "" {
		req.Format = "plain"
	}
	if req.Format != "plain" && req.Format != "html" {
		jsonWrite(w, 400, apiError{id, "invalid_format", "format must be plain or html", nil})
		return
	}
	cfg := a.config.Load()
	if req.Target == nil && cfg.DefaultTarget == "" {
		jsonWrite(w, http.StatusBadRequest, apiError{id, "target_required", "target is required when no default target is configured", nil})
		return
	}
	alias := ""
	if req.Target != nil {
		alias = *req.Target
	}
	name, target, err := cfg.Resolve(alias)
	if err != nil {
		jsonWrite(w, 404, apiError{id, "unknown_target", err.Error(), nil})
		return
	}
	if req.Format == "html" && len([]rune(req.Text)) > telegramLimit {
		jsonWrite(w, 413, apiError{id, "html_too_large", "HTML text exceeds 4096 Unicode characters", nil})
		return
	}
	chunks := []string{req.Text}
	if req.Format == "plain" {
		chunks = splitPlain(req.Text)
	}
	rec := &receipt{id, name, "telegram", len(chunks), 0}
	deliveryCtx, cancel := context.WithTimeout(r.Context(), deliveryTimeout)
	defer cancel()
	for _, chunk := range chunks {
		if err := a.sender.Send(deliveryCtx, target, chunk, req.Format); err != nil {
			a.logger.Warn("notification failed", "request_id", id, "target", name, "format", req.Format, "bytes", len([]byte(req.Text)), "chunks", len(chunks), "duration", time.Since(started), "result", "failure")
			jsonWrite(w, 502, apiError{id, "delivery_failed", "notification delivery failed", rec})
			return
		}
		rec.Delivered++
	}
	a.logger.Info("notification delivered", "request_id", id, "target", name, "format", req.Format, "bytes", len([]byte(req.Text)), "chunks", len(chunks), "duration", time.Since(started), "result", "success")
	jsonWrite(w, 200, rec)
}
