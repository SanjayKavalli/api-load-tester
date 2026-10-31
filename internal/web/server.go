package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go-api-load-tester/internal/config"
	"go-api-load-tester/internal/engine"
	"go-api-load-tester/internal/generator"
)

const maxWebResponseBodyBytes = 2 << 10

//go:embed static/*
var assets embed.FS

type UIOptions struct {
	Mode    string
	Default string
}

func (options UIOptions) normalized() (UIOptions, error) {
	if options.Mode == "" {
		options.Mode = "both"
	}
	if options.Mode != "both" && options.Mode != "classic" && options.Mode != "balanced" {
		return options, fmt.Errorf("ui must be both, classic, or balanced")
	}
	if options.Default == "" {
		options.Default = "classic"
		if options.Mode == "balanced" {
			options.Default = "balanced"
		}
	}
	if options.Default != "classic" && options.Default != "balanced" {
		return options, fmt.Errorf("default-ui must be classic or balanced")
	}
	if options.Mode != "both" && options.Default != options.Mode {
		return options, fmt.Errorf("default-ui must name an enabled interface")
	}
	return options, nil
}

func NewHandlerWithUI(options UIOptions) (http.Handler, error) {
	normalized, err := options.normalized()
	if err != nil {
		return nil, err
	}
	return newHandlerWithUI("", normalized), nil
}

type submission struct {
	Config      config.Config              `json:"config"`
	Files       map[string]json.RawMessage `json:"files"`
	SampleCount int                        `json:"sample_count,omitempty"`
}

type Server struct {
	mu          sync.Mutex
	cancel      context.CancelFunc
	state       string
	message     string
	result      json.RawMessage
	historyPath string
}

func NewHandler() http.Handler {
	return newHandler("")
}

func newHandler(historyPath string) http.Handler {
	options, _ := (UIOptions{}).normalized()
	return newHandlerWithUI(historyPath, options)
}

func newHandlerWithUI(historyPath string, options UIOptions) http.Handler {
	s := &Server{state: "idle", historyPath: historyPath}
	mux := http.NewServeMux()
	workspace := template.Must(template.ParseFS(assets, "static/index.html"))
	enabled := func(layout string) bool { return options.Mode == "both" || options.Mode == layout }
	mux.HandleFunc("GET /api/ui", func(w http.ResponseWriter, r *http.Request) {
		respond(w, http.StatusOK, map[string]any{"classic": enabled("classic"), "balanced": enabled("balanced"), "default": options.Default})
	})
	serveWorkspace := func(layout string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !enabled(layout) {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := workspace.Execute(w, struct{ Balanced bool }{layout == "balanced"}); err != nil {
				return
			}
		}
	}
	mux.HandleFunc("GET /{$}", serveWorkspace(options.Default))
	mux.HandleFunc("GET /index.html", serveWorkspace(options.Default))
	mux.HandleFunc("GET /classic", serveWorkspace("classic"))
	mux.HandleFunc("GET /balanced", serveWorkspace("balanced"))
	mux.HandleFunc("GET /classic/{$}", serveWorkspace("classic"))
	mux.HandleFunc("GET /balanced/{$}", serveWorkspace("balanced"))
	mux.HandleFunc("POST /api/preview", s.preview)
	mux.HandleFunc("POST /api/validate", func(w http.ResponseWriter, r *http.Request) {
		_, dir, err := loadSubmission(w, r)
		if err != nil {
			fail(w, err)
			return
		}
		defer os.RemoveAll(dir)
		respond(w, http.StatusOK, map[string]bool{"valid": true})
	})
	mux.HandleFunc("POST /api/run", s.run)
	mux.HandleFunc("POST /api/stop", s.stop)
	mux.HandleFunc("GET /api/status", s.status)
	static, _ := fs.Sub(assets, "static")
	mux.Handle("/", http.FileServer(http.FS(static)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if index := strings.LastIndex(host, ":"); index >= 0 {
			host = host[:index]
		}
		if host != "127.0.0.1" && host != "localhost" {
			http.Error(w, "local access only", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Host != r.Host || parsed.Scheme != "http" {
				http.Error(w, "cross-origin access denied", http.StatusForbidden)
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func fail(w http.ResponseWriter, err error) {
	respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func loadSubmission(w http.ResponseWriter, r *http.Request) (*config.Config, string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	var input submission
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, "", err
	}
	if input.SampleCount == 0 {
		input.SampleCount = 1
	}
	if input.SampleCount < 1 || input.SampleCount > 10 {
		return nil, "", fmt.Errorf("sample_count must be between 1 and 10")
	}
	if input.Config.UniqueHistoryFile != "" {
		return nil, "", fmt.Errorf("unique history is managed by the server, not uploaded configuration")
	}
	if input.Config.ExecutionMode != "coordinated" && input.Config.ExecutionMode != "multi_api_burst" {
		total := 0
		for _, scenario := range input.Config.Scenarios {
			if scenario.Weight < 1 || scenario.Weight > 100 {
				return nil, "", fmt.Errorf("scenario %q percentage must be a whole number between 1 and 100", scenario.Name)
			}
			total += scenario.Weight
		}
		if total != 100 {
			return nil, "", fmt.Errorf("API percentages must total 100%% (currently %d%%)", total)
		}
	}
	dir, err := os.MkdirTemp("", "loadtest-web-")
	if err != nil {
		return nil, "", err
	}
	success := false
	defer func() {
		if !success {
			os.RemoveAll(dir)
		}
	}()
	for name, data := range input.Files {
		if !safeName(name) || name == "config.json" {
			return nil, "", fmt.Errorf("invalid upload filename %q", name)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			return nil, "", err
		}
	}
	requireFile := func(name string) bool { _, exists := input.Files[name]; return safeName(name) && exists }
	if input.Config.SharedDatasetFile != "" && !requireFile(input.Config.SharedDatasetFile) {
		return nil, "", fmt.Errorf("shared dataset must be uploaded")
	}
	for _, scenario := range input.Config.Scenarios {
		if !requireFile(scenario.BodyFile) || (scenario.DatasetFile != "" && !requireFile(scenario.DatasetFile)) {
			return nil, "", fmt.Errorf("scenario %q requires uploaded body and dataset files", scenario.Name)
		}
		if scenario.TokenEnv != "" || scenario.TokenFile != "" {
			return nil, "", fmt.Errorf("use a scenario Authorization header instead of server token files or environment variables")
		}
	}
	for _, data := range input.Files {
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, "", err
		}
		if hasFilePlaceholder(value) {
			return nil, "", fmt.Errorf("web uploads cannot read server files; embed file content in the JSON before uploading")
		}
	}
	for _, scenario := range input.Config.Scenarios {
		for _, rule := range scenario.FieldRules {
			if strings.Contains(rule.Expression, "file_base64") || strings.Contains(rule.Regex, "file_base64") {
				return nil, "", fmt.Errorf("web rules cannot reference server files")
			}
		}
	}
	data, err := json.Marshal(input.Config)
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return nil, "", err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, "", err
	}
	cfg.PreviewSamples = input.SampleCount
	cfg.DisableGlobalToken = true
	timeout, _ := time.ParseDuration(cfg.Timeout)
	duration, _ := time.ParseDuration(cfg.Duration)
	maxConcurrency := 1000
	if cfg.ExecutionMode == "multi_api_burst" {
		maxConcurrency = 10000
	}
	if timeout <= 0 || timeout > time.Minute || duration > time.Hour || cfg.MaxInFlight > maxConcurrency {
		return nil, "", fmt.Errorf("web limits: duration <= 1h, timeout 1ns..1m, max in flight <= %d", maxConcurrency)
	}
	for _, scenario := range cfg.Scenarios {
		target, err := url.Parse(scenario.URL)
		if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
			return nil, "", fmt.Errorf("scenario %q needs a valid HTTP(S) URL", scenario.Name)
		}
		if len(scenario.ExpectedStatuses) == 0 {
			continue
		}
		for _, status := range scenario.ExpectedStatuses {
			if status < 100 || status > 599 {
				return nil, "", fmt.Errorf("expected statuses must be between 100 and 599")
			}
		}
	}
	success = true
	return cfg, dir, nil
}

func safeName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\:") && filepath.Base(name) == name
}

func hasFilePlaceholder(value any) bool {
	switch item := value.(type) {
	case string:
		return strings.Contains(item, "file_base64:")
	case map[string]any:
		for _, child := range item {
			if hasFilePlaceholder(child) {
				return true
			}
		}
	case []any:
		for _, child := range item {
			if hasFilePlaceholder(child) {
				return true
			}
		}
	}
	return false
}

func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	cfg, dir, err := loadSubmission(w, r)
	if err != nil {
		fail(w, err)
		return
	}
	defer os.RemoveAll(dir)
	bodies := map[string]json.RawMessage{}
	samples := map[string][]json.RawMessage{}
	session, err := generator.NewSession(nil, cfg.DataMode == "existing")
	if err != nil {
		fail(w, err)
		return
	}
	for _, scenario := range cfg.Scenarios {
		record := cfg.PreviewRecord(&scenario)
		for sample := 0; sample < cfg.PreviewSamples; sample++ {
			body, err := generator.BuildBodyWithSession(scenario.BodyTemplate, record, scenario.OverlayDatasetKeys, dir, scenario.FieldRules, session)
			if err != nil {
				fail(w, err)
				return
			}
			if hasFilePlaceholderInBody(body) {
				fail(w, fmt.Errorf("generated payload cannot contain server file references"))
				return
			}
			bodies[scenario.Name] = body
			samples[scenario.Name] = append(samples[scenario.Name], body)
		}
	}
	if cfg.PreviewSamples > 1 {
		respond(w, http.StatusOK, samples)
		return
	}
	respond(w, http.StatusOK, bodies)
}

func hasFilePlaceholderInBody(body []byte) bool {
	return strings.Contains(string(body), "file_base64:")
}

func (s *Server) run(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		respond(w, http.StatusConflict, map[string]string{"error": "a run is already active"})
		return
	}
	cfg, dir, err := loadSubmission(w, r)
	if err != nil {
		fail(w, err)
		return
	}
	cfg.UniqueHistoryFile = s.historyPath
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.state, s.message, s.result = cancel, "running", "", nil
	go func() {
		defer os.RemoveAll(dir)
		defer cancel()
		output := filepath.Join(dir, "results")
		err := engine.RunContextWithBodyLimits(ctx, cfg, false, false, output, maxWebResponseBodyBytes, maxWebResponseBodyBytes)
		var result json.RawMessage
		files, _ := filepath.Glob(filepath.Join(output, "*.json"))
		if len(files) > 0 {
			result, _ = os.ReadFile(files[0])
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.state = "completed"
		if ctx.Err() != nil {
			s.state = "stopped"
		}
		if err != nil {
			s.state, s.message = "failed", err.Error()
		}
		s.result, s.cancel = result, nil
	}()
	respond(w, http.StatusAccepted, map[string]string{"state": s.state})
}

func (s *Server) stop(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.state = "stopping"
	}
	respond(w, http.StatusOK, map[string]string{"state": s.state})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	respond(w, http.StatusOK, map[string]any{"state": s.state, "error": s.message, "result": s.result})
}
