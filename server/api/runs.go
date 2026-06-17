package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"wywy-website/ci/server/orchestrator"

	"wywy-website/ci/server/store"
)

// respondJSON writes v as a JSON response with the given status code.
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// toSlice returns s if non-nil, or an empty slice of T to avoid JSON "null".
func toSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	Store         *store.Store
	Runner        *orchestrator.Runner
	ValidServices map[string]bool // nil = skip validation
	ServicesPath  string          // path to services.txt, empty means not configured
	Broadcaster   *Broadcaster
}

// RegisterRoutes registers API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/runs", h.handleListRuns)
	mux.HandleFunc("GET /api/runs/{id}", h.handleGetRun)
	mux.HandleFunc("GET /api/runs/{id}/logs/{service}", h.handleGetRunLogs)
	mux.HandleFunc("POST /api/runs", h.handleCreateRun)
	mux.HandleFunc("GET /api/services", h.handleListServices)
	mux.HandleFunc("GET /api/runs/{id}/stream", h.handleRunStream)
}

// Service represents a monitored service from services.txt.
type Service struct {
	Name string `json:"name"`
	Repo string `json:"repo"`
	Port int    `json:"port"`
}

func (h *Handler) handleListServices(w http.ResponseWriter, r *http.Request) {
	if h.ServicesPath == "" {
		respondJSON(w, http.StatusOK, []Service{})
		return
	}

	data, err := os.ReadFile(h.ServicesPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var services []Service
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		svc := Service{Name: parts[0], Repo: parts[1]}
		if len(parts) >= 3 {
			svc.Port, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
		}
		services = append(services, svc)
	}

	respondJSON(w, http.StatusOK, toSlice(services))
}

func (h *Handler) handleListRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := h.Store.ListRuns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, toSlice(runs))
}

func (h *Handler) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := h.Store.GetRun(id)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
		} else {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return
	}

	respondJSON(w, http.StatusOK, run)
}

func (h *Handler) handleGetRunLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	service := r.PathValue("service")

	opts := store.LogQueryOpts{}

	if level := r.URL.Query().Get("level"); level != "" {
		opts.Level = level
	}
	if search := r.URL.Query().Get("search"); search != "" {
		opts.Search = search
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			opts.Offset = v
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v >= 0 {
			opts.Limit = v
		}
	}

	entries, err := h.Store.QueryLogEntries(id, service, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, toSlice(entries))
}

func (h *Handler) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Services []string `json:"services"`
		Suite    string   `json:"suite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if h.ValidServices != nil {
		for _, svc := range req.Services {
			if !h.ValidServices[svc] {
				respondJSON(w, http.StatusBadRequest, map[string]string{
					"error": "unknown service: " + svc,
				})
				return
			}
		}
	}

	run, err := h.Runner.StartRun(r.Context(), req.Services, req.Suite)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusCreated, run)
}

