// Package controlplane is the hub's HTTP API: EMR adapters POST canonical
// events here (outbound), and operators query the message archive. This is the
// seam that a future on-demand deployer drives to spin up / configure channels.
package controlplane

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/rollingventures/open-hl7/internal/canonical"
	"github.com/rollingventures/open-hl7/internal/channel"
	"github.com/rollingventures/open-hl7/internal/store"
)

// Server is the control-plane HTTP server.
type Server struct {
	Addr   string
	Router *channel.Router
	Store  store.Store
	Logger *slog.Logger
	// Secret, if set, is required in the X-Hub-Secret header on /events.
	Secret string
}

func (s *Server) log() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// Handler builds the HTTP mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /events", s.events)
	mux.HandleFunc("GET /messages", s.messages)
	return mux
}

// ListenAndServe runs until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{Addr: s.Addr, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	s.log().Info("control-plane listening", "addr", s.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// events ingests a canonical patient and dispatches it as outbound ADT.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	if s.Secret != "" && r.Header.Get("X-Hub-Secret") != s.Secret {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var p canonical.Patient
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if p.Event == "" {
		p.Event = canonical.EventUpdate
	}
	id, err := s.Router.SendPatient(r.Context(), p)
	if err != nil {
		// id may still be set (message stored, send/ack failed).
		writeJSON(w, http.StatusBadGateway, map[string]any{"id": id, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "status": "dispatched"})
}

// messages returns the most recent archived messages.
func (s *Server) messages(w http.ResponseWriter, r *http.Request) {
	msgs, err := s.Store.List(r.Context(), 100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list failed"})
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
