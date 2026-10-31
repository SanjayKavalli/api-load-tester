package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesScenarioTokenEnvironmentBeforeFile(t *testing.T) {
	t.Setenv("LOAD_TEST_TOKEN", "environment-token")
	dir := writeTestConfig(t, "LOAD_TEST_TOKEN", "fallback.token")
	if err := os.WriteFile(filepath.Join(dir, "fallback.token"), []byte("file-token"), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	cfg, err := Load(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Scenarios[0].Headers["Authorization"]; got != "Bearer environment-token" {
		t.Errorf("Authorization = %q, want environment token", got)
	}
}

func TestLoadFallsBackToScenarioTokenFile(t *testing.T) {
	t.Setenv("LOAD_TEST_TOKEN", "")
	dir := writeTestConfig(t, "LOAD_TEST_TOKEN", "fallback.token")
	if err := os.WriteFile(filepath.Join(dir, "fallback.token"), []byte("Bearer file-token\n"), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	cfg, err := Load(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Scenarios[0].Headers["Authorization"]; got != "Bearer file-token" {
		t.Errorf("Authorization = %q, want file token", got)
	}
}

func writeTestConfig(t *testing.T, tokenEnv, tokenFile string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "body.json"), []byte(`{"name":"test"}`), 0o600); err != nil {
		t.Fatalf("write body: %v", err)
	}
	config := map[string]any{
		"duration":     "1s",
		"rate":         1,
		"max_inflight": 1,
		"scenarios": []map[string]any{{
			"name":       "test",
			"weight":     1,
			"method":     "POST",
			"url":        "http://example.test",
			"body_file":  "body.json",
			"token_env":  tokenEnv,
			"token_file": tokenFile,
		}},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return dir
}

func TestValidationEdges(t *testing.T) {
	for _, test := range []struct {
		name, key string
		value     any
	}{
		{"zero timeout", "timeout", "0s"}, {"negative timeout", "timeout", "-1s"}, {"zero burst", "burst_size", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := writeTestConfig(t, "", "")
			path := filepath.Join(dir, "config.json")
			raw, _ := os.ReadFile(path)
			var fields map[string]any
			json.Unmarshal(raw, &fields)
			fields[test.key] = test.value
			raw, _ = json.Marshal(fields)
			os.WriteFile(path, raw, 0600)
			if _, err := Load(path); err == nil {
				t.Fatal("accepted invalid setting")
			}
		})
	}
}

func TestValidationUsesDatasetOverlay(t *testing.T) {
	dir := writeTestConfig(t, "", "")
	os.WriteFile(filepath.Join(dir, "body.json"), []byte(`{"qty":null,"amount":0}`), 0600)
	os.WriteFile(filepath.Join(dir, "dataset.json"), []byte(`[{"qty":2}]`), 0600)
	path := filepath.Join(dir, "config.json")
	raw, _ := os.ReadFile(path)
	var fields map[string]any
	json.Unmarshal(raw, &fields)
	scenario := fields["scenarios"].([]any)[0].(map[string]any)
	scenario["dataset_file"] = "dataset.json"
	scenario["overlay_dataset_keys"] = true
	scenario["field_rules"] = []any{map[string]any{"path": []string{"amount"}, "expression": "body.qty * 10"}}
	raw, _ = json.Marshal(fields)
	os.WriteFile(path, raw, 0600)
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "dataset.json"), []byte(`[null]`), 0600)
	if _, err := Load(path); err == nil {
		t.Fatal("null dataset accepted")
	}
}
