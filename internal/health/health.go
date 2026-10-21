// 文件说明：提供服务状态、依赖明细和健康检查 HTTP 服务。
package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	StatusStarting = "starting"
	StatusReady    = "ready"
	StatusNotReady = "not_ready"
	StatusDraining = "draining"
	StatusStopping = "stopping"
)

type Snapshot struct {
	Service      string            `json:"service"`
	InstanceID   string            `json:"instance_id"`
	Version      string            `json:"version"`
	Status       string            `json:"status"`
	Uptime       string            `json:"uptime"`
	Dependencies map[string]string `json:"dependencies"`
}

type Server struct {
	httpServer *http.Server
	startedAt  time.Time
	version    string
	service    string
	instanceID string
	log        *slog.Logger

	mu           sync.RWMutex
	status       string
	dependencies map[string]string
}

func New(addr, service, instanceID, version string, log *slog.Logger) *Server {
	h := &Server{
		startedAt:    time.Now(),
		version:      version,
		service:      service,
		instanceID:   instanceID,
		log:          log,
		status:       StatusStarting,
		dependencies: map[string]string{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.healthz)
	mux.HandleFunc("/readyz", h.readyz)
	mux.HandleFunc("/version", h.versionz)

	h.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	return h
}

func (s *Server) SetStatus(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func (s *Server) Status() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *Server) SetDependency(name, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dependencies[name] = status
}

func (s *Server) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	deps := make(map[string]string, len(s.dependencies))
	for name, status := range s.dependencies {
		deps[name] = status
	}
	return Snapshot{
		Service:      s.service,
		InstanceID:   s.instanceID,
		Version:      s.version,
		Status:       s.status,
		Uptime:       time.Since(s.startedAt).String(),
		Dependencies: deps,
	}
}

func (s *Server) Start() {
	go func() {
		s.log.Info("health http server starting", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("health http server stopped", "error", err)
		}
	}()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Snapshot())
}

func (s *Server) readyz(w http.ResponseWriter, _ *http.Request) {
	snapshot := s.Snapshot()
	if snapshot.Status != StatusReady {
		writeJSON(w, http.StatusServiceUnavailable, snapshot)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) versionz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service":     s.service,
		"instance_id": s.instanceID,
		"version":     s.version,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
