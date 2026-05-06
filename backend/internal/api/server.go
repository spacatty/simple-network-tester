package api

import (
	"context"
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"loadtester/backend/internal/config"
	"loadtester/backend/internal/model"
	"loadtester/backend/internal/proxy"
	"loadtester/backend/internal/runner"
	"loadtester/backend/internal/storage"
)

type Server struct {
	store     *storage.Store
	runner    *runner.Service
	proxies   *proxy.Manager
	filePath  string
	authToken string
}

func NewServer(store *storage.Store, runs *runner.Service, proxies *proxy.Manager, filesDir string, authToken string) *Server {
	return &Server{store: store, runner: runs, proxies: proxies, filePath: filesDir, authToken: authToken}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.health)
	mux.HandleFunc("/api/files/upload", s.uploadFile)
	mux.HandleFunc("/api/proxies/import", s.importProxies)
	mux.HandleFunc("/api/proxies", s.listProxies)
	mux.HandleFunc("/api/proxies/recheck", s.recheckProxies)
	mux.HandleFunc("/api/proxies/delete-inactive", s.deleteInactiveProxies)
	mux.HandleFunc("/api/proxies/enabled", s.setProxyEnabled)
	mux.HandleFunc("/api/runs/start", s.startRun)
	mux.HandleFunc("/api/runs/stream", s.streamRun)
	mux.HandleFunc("/api/reports", s.listReports)
	mux.HandleFunc("/api/reports/export", s.exportReportsCSV)
	return cors(s.auth(mux))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	defer file.Close()

	id := fmt.Sprintf("f_%d", time.Now().UnixNano())
	dstPath := filepath.Join(s.filePath, id+"_"+header.Filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer dst.Close()
	if _, err := dst.ReadFrom(file); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.SaveUploadedFile(r.Context(), id, header.Filename, dstPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"fileId": id, "fileName": header.Filename})
}

func (s *Server) importProxies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		List string `json:"list"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	items := proxy.ParseProxyList(body.List)
	if err := s.store.UpsertProxies(r.Context(), items); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"imported": len(items)})
}

func (s *Server) listProxies(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListProxies(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) recheckProxies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	list, err := s.proxies.ValidateAndPersist(r.Context(), 5*time.Second)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) deleteInactiveProxies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	deleted, err := s.store.DeleteInactiveProxies(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": deleted})
}

func (s *Server) setProxyEnabled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.SetAllProxiesEnabled(r.Context(), body.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) startRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var cfg model.LoadTestConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := validateConfig(cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	config.NormalizeConfig(&cfg)
	runID := runner.NewRunID()
	s.runner.Start(runID, cfg)
	writeJSON(w, http.StatusOK, map[string]string{"runId": runID})
}

func (s *Server) streamRun(w http.ResponseWriter, r *http.Request) {
	runID := r.URL.Query().Get("runId")
	if strings.TrimSpace(runID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing runId"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stream unsupported"})
		return
	}
	ch := s.runner.Subscribe(runID)
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(update)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) listReports(w http.ResponseWriter, r *http.Request) {
	reports, err := s.store.ListRunSummaries(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, reports)
}

func (s *Server) exportReportsCSV(w http.ResponseWriter, r *http.Request) {
	reportID := r.URL.Query().Get("runId")
	if reportID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing runId"})
		return
	}
	summary, err := s.store.RunSummary(context.Background(), reportID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="report_`+reportID+`.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"run_id", summary.RunID})
	_ = cw.Write([]string{"total_requests", strconv.Itoa(summary.TotalRequests)})
	_ = cw.Write([]string{"success_count", strconv.Itoa(summary.SuccessCount)})
	_ = cw.Write([]string{"error_count", strconv.Itoa(summary.ErrorCount)})
	_ = cw.Write([]string{"avg_latency_ms", fmt.Sprintf("%.2f", summary.AvgLatencyMS)})
	_ = cw.Write([]string{"p95_latency_ms", strconv.FormatInt(summary.P95LatencyMS, 10)})
	_ = cw.Write([]string{"rps", fmt.Sprintf("%.2f", summary.RPS)})
	cw.Flush()
}

func validateConfig(cfg model.LoadTestConfig) error {
	if strings.TrimSpace(cfg.URL) == "" {
		return errors.New("url is required")
	}
	if cfg.Method != "" && !strings.EqualFold(cfg.Method, http.MethodGet) && !strings.EqualFold(cfg.Method, http.MethodPost) {
		return errors.New("only GET and POST are supported")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}

		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.authToken)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bearer token"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
