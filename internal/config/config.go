package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-api-load-tester/internal/generator"
)

type Config struct {
	Duration            string           `json:"duration"`
	Rate                int              `json:"rate"` // bursts per second
	BurstSize           int              `json:"burst_size"`
	MaxInFlight         int              `json:"max_inflight"`
	Timeout             string           `json:"timeout"`
	ExecutionMode       string           `json:"execution_mode,omitempty"` // weighted or coordinated
	BurstMode           string           `json:"burst_mode,omitempty"`
	BurstInterval       string           `json:"burst_interval,omitempty"`
	DataMode            string           `json:"data_mode,omitempty"`
	UniqueHistoryFile   string           `json:"unique_history_file,omitempty"`
	RecordRequestBodies bool             `json:"record_request_bodies,omitempty"`
	SharedDatasetFile   string           `json:"shared_dataset_file,omitempty"`
	SharedDatasetMode   string           `json:"shared_dataset_mode,omitempty"` // fixed, random, or cycle
	Scenarios           []Scenario       `json:"scenarios"`
	BaseDir             string           `json:"-"`
	SharedDataset       []map[string]any `json:"-"`
	PreviewSamples      int              `json:"-"`
	DisableGlobalToken  bool             `json:"-"`
}

type Scenario struct {
	Name               string                `json:"name"`
	Weight             int                   `json:"weight"`
	Method             string                `json:"method"`
	URL                string                `json:"url"`
	BodyFile           string                `json:"body_file"`
	DatasetFile        string                `json:"dataset_file,omitempty"`
	DatasetMode        string                `json:"dataset_mode,omitempty"` // random or cycle
	OverlayDatasetKeys bool                  `json:"overlay_dataset_keys"`
	Headers            map[string]string     `json:"headers,omitempty"`
	TokenEnv           string                `json:"token_env,omitempty"`
	TokenFile          string                `json:"token_file,omitempty"`
	ExpectedStatuses   []int                 `json:"expected_statuses,omitempty"`
	FieldRules         []generator.FieldRule `json:"field_rules,omitempty"`

	BodyTemplate map[string]any   `json:"-"`
	Dataset      []map[string]any `json:"-"`
	DatasetIndex int              `json:"-"`
}

func (cfg *Config) UsesSharedRecords() bool {
	return cfg.ExecutionMode == "coordinated" || cfg.ExecutionMode == "multi_api_burst"
}

func (cfg *Config) PreviewRecord(s *Scenario) map[string]any {
	if cfg.UsesSharedRecords() {
		if len(cfg.SharedDataset) > 0 {
			return cfg.SharedDataset[0]
		}
		return nil
	}
	if len(s.Dataset) > 0 {
		return s.Dataset[0]
	}
	return nil
}

func validateDataset(records []map[string]any) error {
	if len(records) == 0 {
		return fmt.Errorf("dataset is empty")
	}
	for index, record := range records {
		if record == nil {
			return fmt.Errorf("dataset record %d must be a non-null object", index+1)
		}
	}
	return nil
}

func Load(path string) (*Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := generator.ValidateJSONNumbers(raw); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config JSON: %w", err)
	}
	cfg.BaseDir = filepath.Dir(absPath)
	if cfg.DataMode == "" {
		cfg.DataMode = "fresh"
	}
	if cfg.DataMode != "fresh" && cfg.DataMode != "existing" {
		return nil, fmt.Errorf("data_mode must be fresh or existing")
	}
	if cfg.UniqueHistoryFile != "" {
		cfg.UniqueHistoryFile = resolvePath(cfg.BaseDir, cfg.UniqueHistoryFile)
	}

	if cfg.Duration == "" {
		cfg.Duration = "1m"
	}
	if cfg.Timeout == "" {
		cfg.Timeout = "30s"
	}
	if cfg.Rate < 1 || cfg.Rate > 10000 {
		return nil, fmt.Errorf("rate must be between 1 and 10000")
	}
	if cfg.BurstSize == 0 {
		var fields map[string]json.RawMessage
		json.Unmarshal(raw, &fields)
		if _, supplied := fields["burst_size"]; supplied {
			return nil, fmt.Errorf("burst_size must be greater than zero")
		}
		cfg.BurstSize = 1
	}
	if cfg.BurstSize < 1 || cfg.BurstSize > 100000 {
		return nil, fmt.Errorf("burst_size must be between 1 and 100000")
	}
	if cfg.MaxInFlight < 1 {
		return nil, fmt.Errorf("max_inflight must be greater than zero")
	}
	if _, err := time.ParseDuration(cfg.Duration); err != nil {
		return nil, fmt.Errorf("invalid duration %q: %w", cfg.Duration, err)
	}
	if d, _ := time.ParseDuration(cfg.Duration); d <= 0 {
		return nil, fmt.Errorf("duration must be greater than zero")
	}
	if timeout, err := time.ParseDuration(cfg.Timeout); err != nil || timeout <= 0 {
		return nil, fmt.Errorf("timeout must be a positive duration, got %q", cfg.Timeout)
	}
	if len(cfg.Scenarios) == 0 {
		return nil, fmt.Errorf("at least one scenario is required")
	}
	if cfg.ExecutionMode == "" {
		cfg.ExecutionMode = "weighted"
	}
	switch cfg.ExecutionMode {
	case "weighted", "coordinated", "multi_api_burst":
	default:
		return nil, fmt.Errorf("execution_mode must be weighted, coordinated, or multi_api_burst")
	}
	if cfg.ExecutionMode == "multi_api_burst" {
		if cfg.BurstMode == "" {
			cfg.BurstMode = "once"
		}
		if cfg.BurstMode != "once" && cfg.BurstMode != "repeat" {
			return nil, fmt.Errorf("burst_mode must be once or repeat")
		}
		if cfg.BurstInterval == "" {
			cfg.BurstInterval = "1s"
		}
		interval, err := time.ParseDuration(cfg.BurstInterval)
		if err != nil || interval < time.Millisecond {
			return nil, fmt.Errorf("burst_interval must be at least 1ms")
		}
		if cfg.BurstSize > cfg.MaxInFlight/len(cfg.Scenarios) {
			return nil, fmt.Errorf("max_inflight must cover users per burst multiplied by the number of APIs")
		}
	}
	if cfg.SharedDatasetMode == "" {
		cfg.SharedDatasetMode = "fixed"
	}
	switch cfg.SharedDatasetMode {
	case "fixed", "random", "cycle":
	default:
		return nil, fmt.Errorf("shared_dataset_mode must be fixed, random, or cycle")
	}
	if cfg.SharedDatasetFile != "" {
		datasetPath := resolvePath(cfg.BaseDir, cfg.SharedDatasetFile)
		data, err := os.ReadFile(datasetPath)
		if err != nil {
			return nil, fmt.Errorf("shared_dataset_file: %w", err)
		}
		if err := json.Unmarshal(data, &cfg.SharedDataset); err != nil {
			return nil, fmt.Errorf("shared dataset must be a JSON array of objects: %w", err)
		}
		if err := generator.ValidateJSONNumbers(data); err != nil {
			return nil, err
		}
		if err := validateDataset(cfg.SharedDataset); err != nil {
			return nil, fmt.Errorf("shared %w", err)
		}
	}

	seen := map[string]bool{}
	for i := range cfg.Scenarios {
		s := &cfg.Scenarios[i]
		if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.URL) == "" || strings.TrimSpace(s.BodyFile) == "" {
			return nil, fmt.Errorf("scenario %d needs name, url, and body_file", i+1)
		}
		if seen[s.Name] {
			return nil, fmt.Errorf("duplicate scenario name %q", s.Name)
		}
		seen[s.Name] = true
		if s.Weight < 1 {
			return nil, fmt.Errorf("scenario %q weight must be greater than zero", s.Name)
		}
		s.Method = strings.ToUpper(strings.TrimSpace(s.Method))
		if s.Method == "" {
			s.Method = "POST"
		}
		if err := resolveScenarioAuthorization(s, cfg.BaseDir); err != nil {
			return nil, err
		}
		switch s.DatasetMode {
		case "", "random", "cycle":
		default:
			return nil, fmt.Errorf("scenario %q dataset_mode must be random or cycle", s.Name)
		}
		if s.DatasetFile != "" {
			datasetPath := resolvePath(cfg.BaseDir, s.DatasetFile)
			data, err := os.ReadFile(datasetPath)
			if err != nil {
				return nil, fmt.Errorf("scenario %q dataset_file: %w", s.Name, err)
			}
			if err := json.Unmarshal(data, &s.Dataset); err != nil {
				return nil, fmt.Errorf("scenario %q dataset must be a JSON array of objects: %w", s.Name, err)
			}
			if err := generator.ValidateJSONNumbers(data); err != nil {
				return nil, err
			}
			if err := validateDataset(s.Dataset); err != nil {
				return nil, fmt.Errorf("scenario %q: %w", s.Name, err)
			}
			if s.DatasetMode == "" {
				s.DatasetMode = "random"
			}
		}
		bodyPath := resolvePath(cfg.BaseDir, s.BodyFile)
		bodyData, err := os.ReadFile(bodyPath)
		if err != nil {
			return nil, fmt.Errorf("scenario %q body_file: %w", s.Name, err)
		}
		if err := json.Unmarshal(bodyData, &s.BodyTemplate); err != nil {
			return nil, fmt.Errorf("scenario %q body must be a JSON object: %w", s.Name, err)
		}
		if err := generator.ValidateJSONNumbers(bodyData); err != nil {
			return nil, err
		}
		if s.BodyTemplate == nil {
			return nil, fmt.Errorf("scenario %q body must be a JSON object", s.Name)
		}
		if len(s.FieldRules) > 0 {
			session, err := generator.NewSession(nil, cfg.DataMode == "existing")
			if err != nil {
				return nil, err
			}
			if _, err := generator.BuildBodyWithSession(s.BodyTemplate, cfg.PreviewRecord(s), s.OverlayDatasetKeys, cfg.BaseDir, s.FieldRules, session); err != nil {
				return nil, fmt.Errorf("scenario %q field rules: %w", s.Name, err)
			}
		}
	}
	return &cfg, nil
}

func resolveScenarioAuthorization(s *Scenario, baseDir string) error {
	if strings.TrimSpace(s.TokenEnv) == "" && strings.TrimSpace(s.TokenFile) == "" {
		return nil
	}
	if authorizationHeader(s.Headers) != "" {
		return fmt.Errorf("scenario %q cannot combine Authorization header with token_env or token_file", s.Name)
	}

	token := ""
	if envName := strings.TrimSpace(s.TokenEnv); envName != "" {
		token = strings.TrimSpace(os.Getenv(envName))
	}
	if token == "" && strings.TrimSpace(s.TokenFile) != "" {
		tokenPath := resolvePath(baseDir, s.TokenFile)
		raw, err := os.ReadFile(tokenPath)
		if err != nil {
			return fmt.Errorf("scenario %q token_file: %w", s.Name, err)
		}
		token = strings.TrimSpace(string(raw))
	}
	if token == "" {
		return fmt.Errorf("scenario %q has no token in environment variable %q or token_file %q", s.Name, s.TokenEnv, s.TokenFile)
	}

	if s.Headers == nil {
		s.Headers = make(map[string]string)
	}
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		s.Headers["Authorization"] = token
	} else {
		s.Headers["Authorization"] = "Bearer " + token
	}
	return nil
}

func authorizationHeader(headers map[string]string) string {
	for key, value := range headers {
		if strings.EqualFold(key, "Authorization") {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func resolvePath(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(base, path))
}
