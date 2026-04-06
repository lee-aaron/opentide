package registry

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/opentide/opentide/pkg/skillspec"
)

// Server is the HTTP API for the skill registry.
type Server struct {
	registry *Registry
	logger   *slog.Logger
	mux      *http.ServeMux
}

// NewServer creates a registry HTTP server.
func NewServer(registry *Registry, logger *slog.Logger) *Server {
	s := &Server{
		registry: registry,
		logger:   logger,
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /v1/skills", s.handleSearch)
	s.mux.HandleFunc("GET /v1/skills/{name}", s.handleGet)
	s.mux.HandleFunc("GET /v1/skills/{name}/{version}", s.handleGetVersion)
	s.mux.HandleFunc("POST /v1/skills", s.handlePublish)
	s.mux.HandleFunc("DELETE /v1/skills/{name}/{version}", s.handleDelete)
	s.mux.HandleFunc("POST /v1/skills/{name}/install", s.handleInstall)
	s.mux.HandleFunc("POST /v1/skills/{name}/{version}/install", s.handleInstallVersion)
	s.mux.HandleFunc("GET /v1/skills/{name}/versions", s.handleListVersions)
	s.mux.HandleFunc("GET /health", s.handleHealth)
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// PublishRequest is the body for POST /v1/skills.
type PublishRequest struct {
	Signed   skillspec.SignedManifest `json:"signed"`
	ImageRef string                  `json:"image_ref"`
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	var req PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ImageRef == "" {
		s.jsonError(w, "image_ref is required", http.StatusBadRequest)
		return
	}

	if err := s.registry.Publish(r.Context(), &req.Signed, req.ImageRef); err != nil {
		if strings.Contains(err.Error(), "signature verification failed") {
			s.jsonError(w, err.Error(), http.StatusForbidden)
			return
		}
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.logger.Info("skill published",
		"name", req.Signed.Manifest.Name,
		"version", req.Signed.Manifest.Version,
		"author", req.Signed.Manifest.Author)

	s.jsonOK(w, map[string]string{
		"status":  "published",
		"name":    req.Signed.Manifest.Name,
		"version": req.Signed.Manifest.Version,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := SearchQuery{
		Term:   r.URL.Query().Get("q"),
		Author: r.URL.Query().Get("author"),
	}

	result, err := s.registry.Search(r.Context(), q)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonOK(w, result)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	entry, err := s.registry.Get(r.Context(), name, "")
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	s.jsonOK(w, entry)
}

func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")
	entry, err := s.registry.Get(r.Context(), name, version)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	s.jsonOK(w, entry)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")
	if err := s.registry.store.Delete(r.Context(), name, version); err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	s.logger.Info("skill deleted", "name", name, "version", version)
	s.jsonOK(w, map[string]string{"status": "deleted"})
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	entry, err := s.registry.Install(r.Context(), name, "")
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	s.jsonOK(w, entry)
}

func (s *Server) handleInstallVersion(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")
	entry, err := s.registry.Install(r.Context(), name, version)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	s.jsonOK(w, entry)
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	versions, err := s.registry.store.ListVersions(r.Context(), name)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	s.jsonOK(w, versions)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s.jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
