package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go-api-load-tester/internal/config"
	"go-api-load-tester/internal/generator"
	"go-api-load-tester/internal/report"
)

func TestTransportTruncationIsVisibleInUnlimitedReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat("x", 3<<20))) }))
	defer server.Close()
	dir := t.TempDir()
	cfg := &config.Config{Duration: "1s", Timeout: "1s", Rate: 1, BurstSize: 1, MaxInFlight: 1, ExecutionMode: "multi_api_burst", Scenarios: []config.Scenario{{Name: "large", Weight: 1, URL: server.URL, Method: "GET", BodyTemplate: map[string]any{}}}}
	if err := RunContext(context.Background(), cfg, false, false, dir, 0); err != nil {
		t.Fatal(err)
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	raw, _ := os.ReadFile(paths[0])
	var result struct {
		Responses []report.ResponseRecord `json:"responses"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Responses) != 1 || !result.Responses[0].Truncated || result.Responses[0].BodyBytes != 2<<20 {
		t.Fatal("transport truncation was hidden")
	}
}

func TestScenarioCycleIsOrdered(t *testing.T) {
	cfg := &config.Config{BurstSize: 1, Scenarios: []config.Scenario{{Weight: 100, DatasetMode: "cycle", Dataset: []map[string]any{{"id": "A"}, {"id": "B"}, {"id": "C"}}}}}
	index := 0
	for request := 0; request < 9; request++ {
		got := scheduleBurst(cfg, 100, &index)[0].record["id"]
		if got != []string{"A", "B", "C"}[request%3] {
			t.Fatal("cycle is not ordered", got)
		}
	}
}

func TestGenerationFailureIsNotSuccessful(t *testing.T) {
	cfg := &config.Config{Duration: "15ms", Timeout: "1s", Rate: 1000, BurstSize: 1, MaxInFlight: 1, Scenarios: []config.Scenario{{Name: "invalid", Weight: 100, URL: "http://127.0.0.1:1", Method: "POST", BodyTemplate: map[string]any{"ssn_passport": "{{ssn}}", "dob": "bad", "gender": "male"}}}}
	if err := RunContext(context.Background(), cfg, false, false, "", 0); err == nil {
		t.Fatal("a run that generated no valid requests succeeded")
	}
}

func TestInvalidMultiAPIBurstSendsNothing(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	cfg := &config.Config{Duration: "1s", Timeout: "1s", Rate: 1, BurstSize: 1, MaxInFlight: 2, ExecutionMode: "multi_api_burst", UniqueHistoryFile: filepath.Join(t.TempDir(), "unique.jsonl")}
	for _, name := range []string{"first", "second"} {
		cfg.Scenarios = append(cfg.Scenarios, config.Scenario{Name: name, Weight: 1, URL: server.URL, Method: "POST", BodyTemplate: map[string]any{"id": ""}, FieldRules: []generator.FieldRule{{Path: []string{"id"}, Regex: "ONLY", Unique: true}}})
	}
	if err := RunContext(context.Background(), cfg, false, false, "", 0); err == nil {
		t.Fatal("insufficient unique capacity was accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("sent a partial synchronized burst")
	}
}

func TestRepeatedMultiAPIBurstsContinueUntilDuration(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	cfg := &config.Config{Duration: "150ms", Timeout: "1s", Rate: 1, BurstSize: 1, MaxInFlight: 2, ExecutionMode: "multi_api_burst", BurstMode: "repeat", BurstInterval: "10ms"}
	for _, name := range []string{"first", "second"} {
		cfg.Scenarios = append(cfg.Scenarios, config.Scenario{Name: name, Weight: 1, URL: server.URL, Method: "POST", BodyTemplate: map[string]any{}})
	}
	if err := RunContext(context.Background(), cfg, false, false, "", 0); err != nil {
		t.Fatal(err)
	}
	if calls.Load() < 4 || calls.Load()%2 != 0 {
		t.Fatal("expected multiple complete two-API groups", calls.Load())
	}
}

func TestRepeatedRunsCaptureActualBodies(t *testing.T) {
	sent := make(chan map[string]any, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		sent <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	dir := t.TempDir()
	cfg := &config.Config{Duration: "1s", Timeout: "1s", Rate: 1, BurstSize: 1, MaxInFlight: 1, ExecutionMode: "multi_api_burst", RecordRequestBodies: true, UniqueHistoryFile: filepath.Join(dir, "unique.jsonl"), Scenarios: []config.Scenario{{Name: "test", Weight: 1, URL: server.URL, Method: "POST", BodyTemplate: map[string]any{"id": ""}, FieldRules: []generator.FieldRule{{Path: []string{"id"}, Expression: `run_id + "-" + string(sequence)`, Unique: true}}}}}
	seen := map[string]bool{}
	for run := 0; run < 2; run++ {
		if err := RunContext(context.Background(), cfg, false, false, filepath.Join(dir, "reports"), 0); err != nil {
			t.Fatal(err)
		}
		body := <-sent
		if seen[body["id"].(string)] {
			t.Fatal("request ID repeated")
		}
		seen[body["id"].(string)] = true
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "reports", "*.json"))
	if len(paths) != 2 {
		t.Fatalf("got %d reports, want 2", len(paths))
	}
	for _, path := range paths {
		data, _ := os.ReadFile(path)
		var result struct {
			Responses []report.ResponseRecord `json:"responses"`
			Summary   report.RunSummary       `json:"summary"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Responses) != 1 || result.Summary.RunID == "" {
			t.Fatal(string(data))
		}
		var captured map[string]any
		json.Unmarshal(result.Responses[0].RequestBody, &captured)
		if !seen[captured["id"].(string)] || result.Responses[0].RunID != result.Summary.RunID {
			t.Fatal("recorded body differs from sent body")
		}
	}
}

func TestMultiAPIBurstReleasesAllRequestsTogether(t *testing.T) {
	arrived := make(chan bool, 6)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { arrived <- true; <-release; w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	defer close(release)
	cfg := &config.Config{Duration: "5s", Timeout: "2s", Rate: 1, BurstSize: 2, MaxInFlight: 6, ExecutionMode: "multi_api_burst", BurstMode: "once"}
	for _, name := range []string{"create", "update", "search"} {
		cfg.Scenarios = append(cfg.Scenarios, config.Scenario{Name: name, Weight: 1, URL: server.URL, Method: "POST", BodyTemplate: map[string]any{"id": "shared"}})
	}
	finished := make(chan error, 1)
	go func() { finished <- RunContext(context.Background(), cfg, false, false, "", 0) }()
	for index := 0; index < 6; index++ {
		select {
		case <-arrived:
		case <-time.After(time.Second):
			t.Fatal("requests were not released concurrently")
		}
	}
	for index := 0; index < 6; index++ {
		release <- struct{}{}
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("single burst waited for duration or repeated")
	}
}

func TestScheduleBurstCoordinatesAllScenariosWithSameRecord(t *testing.T) {
	cfg := &config.Config{
		BurstSize:         2,
		ExecutionMode:     "coordinated",
		SharedDatasetMode: "cycle",
		SharedDataset: []map[string]any{
			{"person_id": "person-1"},
			{"person_id": "person-2"},
		},
		Scenarios: []config.Scenario{
			{Name: "role_a"},
			{Name: "role_b"},
			{Name: "role_c"},
		},
	}

	index := 0
	requests := scheduleBurst(cfg, 0, &index)
	if len(requests) != 6 {
		t.Fatalf("scheduled %d requests, want 6", len(requests))
	}

	for user := 0; user < 2; user++ {
		wantPerson := "person-" + string(rune('1'+user))
		for scenario := 0; scenario < 3; scenario++ {
			request := requests[user*3+scenario]
			if got := request.record["person_id"]; got != wantPerson {
				t.Errorf("user %d scenario %d person_id = %v, want %s", user, scenario, got, wantPerson)
			}
		}
	}
}

func TestScheduleBurstUsesFixedRecordForContention(t *testing.T) {
	cfg := &config.Config{
		BurstSize:         3,
		ExecutionMode:     "coordinated",
		SharedDatasetMode: "fixed",
		SharedDataset: []map[string]any{
			{"person_id": "contended-person"},
			{"person_id": "unused-person"},
		},
		Scenarios: []config.Scenario{{Name: "update"}},
	}

	index := 0
	requests := scheduleBurst(cfg, 0, &index)
	for i, request := range requests {
		if got := request.record["person_id"]; got != "contended-person" {
			t.Errorf("request %d person_id = %v, want contended-person", i, got)
		}
	}
}
