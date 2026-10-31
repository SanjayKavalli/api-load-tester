package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go-api-load-tester/internal/config"
	"go-api-load-tester/internal/generator"
)

func testSubmission() submission {
	return submission{Config: config.Config{
		Duration: "20ms", Rate: 100, BurstSize: 1, MaxInFlight: 2, Timeout: "1s",
		Scenarios: []config.Scenario{{Name: "create", Weight: 100, Method: "POST", URL: "http://127.0.0.1:1", BodyFile: "body.json", OverlayDatasetKeys: true,
			FieldRules: []generator.FieldRule{{Path: []string{"id"}, Regex: `PT-[0-9]{4}`}, {Path: []string{"label"}, Expression: `body.id + "-ok"`}}}},
	}, Files: map[string]json.RawMessage{"body.json": json.RawMessage(`{"id":"", "label":""}`)}}
}

func request(handler http.Handler, method, path string, input any) *httptest.ResponseRecorder {
	data, _ := json.Marshal(input)
	r := httptest.NewRequest(method, "http://localhost"+path, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestEndUserManualServed(t *testing.T) {
	handler := NewHandler()
	manual := request(handler, "GET", "/user-manual.html", nil)
	if manual.Code != http.StatusOK || !strings.HasPrefix(manual.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("manual: %d %s", manual.Code, manual.Body)
	}
	for _, content := range []string{"Load Lab | User Manual", `id="quick-start"`, `id="percentages"`, `id="field-rules"`, `id="troubleshooting"`} {
		if !strings.Contains(manual.Body.String(), content) {
			t.Errorf("manual is missing %q", content)
		}
	}
	home := request(handler, "GET", "/", nil)
	if !strings.Contains(home.Body.String(), `href="/user-manual.html"`) {
		t.Fatal("workspace does not link to the end-user manual")
	}
}

func TestImportLabelTargetsSetupFileInput(t *testing.T) {
	home := request(NewHandler(), "GET", "/", nil)
	if home.Code != http.StatusOK || !strings.Contains(home.Body.String(), `for="import-config"`) {
		t.Fatal("Import setup must explicitly target its file input so contextual help cannot become the label's control")
	}
}

func TestReadOnlyAnalysisAndMockupRoutes(t *testing.T) {
	handler := NewHandler()
	for _, path := range []string{"/analysis.html", "/analysis.js", "/snapshots.js", "/ui-core.js", "/interface-options.html", "/mockups.css"} {
		if response := request(handler, "GET", path, nil); response.Code != http.StatusOK {
			t.Fatal(path, response.Code)
		}
	}
}

func TestWorkspaceLayoutConfiguration(t *testing.T) {
	for _, test := range []struct {
		mode, preferred   string
		classic, balanced int
	}{
		{"both", "classic", 200, 200}, {"both", "balanced", 200, 200}, {"classic", "", 200, 404}, {"balanced", "", 404, 200},
	} {
		t.Run(test.mode+test.preferred, func(t *testing.T) {
			handler, err := NewHandlerWithUI(UIOptions{Mode: test.mode, Default: test.preferred})
			if err != nil {
				t.Fatal(err)
			}
			for path, status := range map[string]int{"/classic": test.classic, "/balanced": test.balanced} {
				response := request(handler, "GET", path, nil)
				if response.Code != status {
					t.Fatal(path, response.Code)
				}
				if status == 200 && strings.Contains(response.Body.String(), `class="balanced-page"`) != (path == "/balanced") {
					t.Fatal("wrong layout", path)
				}
			}
			root := request(handler, "GET", "/", nil)
			if root.Code != 200 {
				t.Fatal(root.Code)
			}
		})
	}
	if _, err := NewHandlerWithUI(UIOptions{Mode: "classic", Default: "balanced"}); err == nil {
		t.Fatal("disabled default accepted")
	}
}

func TestWorkspaceLayoutsShareControlsAndScripts(t *testing.T) {
	handler := NewHandler()
	for _, path := range []string{"/classic", "/balanced"} {
		response := request(handler, "GET", path, nil)
		for _, id := range []string{"start", "stop", "scenario-tabs", "scenario-editor", "field-rules", "import-config", "preview", "results", "shared-file"} {
			if strings.Count(response.Body.String(), `id="`+id+`"`) != 1 {
				t.Fatalf("%s has missing or duplicate shared control %s", path, id)
			}
		}
		for _, script := range []string{"/ui-core.js", "/app.js", "/snapshots.js"} {
			if !strings.Contains(response.Body.String(), script) {
				t.Fatal("shared behavior script missing", path, script)
			}
		}
	}
	for _, path := range []string{"/balanced.css", "/balanced.js", "/ui-switch.js", "/ui-switch.css", "/api/ui"} {
		if response := request(handler, "GET", path, nil); response.Code != 200 {
			t.Fatal(path, response.Code)
		}
	}
}

func TestImportValidationDoesNotSendRequests(t *testing.T) {
	var calls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer target.Close()
	input := testSubmission()
	input.Config.Scenarios[0].URL = target.URL
	if response := request(NewHandler(), "POST", "/api/validate", input); response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	input.Config.Scenarios[0].FieldRules[0].Regex = "["
	if response := request(NewHandler(), "POST", "/api/validate", input); response.Code != 400 {
		t.Fatal("invalid regex import accepted", response.Code)
	}
	if calls.Load() != 0 {
		t.Fatal("validation sent requests")
	}
}

func TestSynchronizedPreviewDoesNotUseIgnoredScenarioDataset(t *testing.T) {
	input := testSubmission()
	input.Config.ExecutionMode = "multi_api_burst"
	input.Config.Scenarios[0].FieldRules = nil
	input.Config.Scenarios[0].DatasetFile = "records.json"
	input.Files["body.json"] = json.RawMessage(`{"id":"TEMPLATE"}`)
	input.Files["records.json"] = json.RawMessage(`[{"id":"DATASET"}]`)
	response := request(NewHandler(), "POST", "/api/preview", input)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "TEMPLATE") || strings.Contains(response.Body.String(), "DATASET") {
		t.Fatal(response.Code, response.Body)
	}
	input.Config.SharedDatasetFile = "records.json"
	response = request(NewHandler(), "POST", "/api/preview", input)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "DATASET") {
		t.Fatal(response.Code, response.Body)
	}
}

func TestWebRejectsInvalidDataAndDoesNotInheritServerToken(t *testing.T) {
	t.Setenv("API_TOKEN", "audit-nonsecret")
	input := testSubmission()
	for _, record := range []json.RawMessage{json.RawMessage(`[null]`), json.RawMessage(`[{"id":9007199254740993}]`)} {
		input.Config.Scenarios[0].DatasetFile = "records.json"
		input.Files["records.json"] = record
		if response := request(NewHandler(), "POST", "/api/preview", input); response.Code != 400 {
			t.Fatal("invalid dataset accepted", response.Code)
		}
	}
	input = testSubmission()
	raw, _ := json.Marshal(input)
	req := httptest.NewRequest("POST", "http://localhost/api/run", bytes.NewReader(raw))
	recorder := httptest.NewRecorder()
	cfg, dir, err := loadSubmission(recorder, req)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if !cfg.DisableGlobalToken {
		t.Fatal("web configuration inherits global authentication")
	}
}

func TestMultiAPIBurstSamplePreviews(t *testing.T) {
	input := testSubmission()
	input.Config.ExecutionMode = "multi_api_burst"
	input.Config.BurstMode = "once"
	input.Config.MaxInFlight = 2
	second := input.Config.Scenarios[0]
	second.Name = "update"
	second.Weight = 1
	input.Config.Scenarios = append(input.Config.Scenarios, second)
	input.Config.Scenarios[0].Weight = 1
	input.SampleCount = 3
	w := request(NewHandler(), "POST", "/api/preview", input)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body)
	}
	var samples map[string][]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &samples); err != nil {
		t.Fatal(err)
	}
	if len(samples["create"]) != 3 || len(samples["update"]) != 3 {
		t.Fatal(samples)
	}
	input.Config.MaxInFlight = 1
	if w := request(NewHandler(), "POST", "/api/run", input); w.Code != http.StatusBadRequest {
		t.Fatal("insufficient capacity accepted", w.Code)
	}
	input.Config.MaxInFlight = 2
	input.Config.UniqueHistoryFile = "secret.jsonl"
	if w := request(NewHandler(), "POST", "/api/preview", input); w.Code != http.StatusBadRequest {
		t.Fatal("client-selected history path accepted", w.Code)
	}
}

func TestWebRepeatedRunHistoryAndExplicitReuse(t *testing.T) {
	var calls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(http.StatusCreated) }))
	defer target.Close()
	input := testSubmission()
	input.Config.ExecutionMode = "multi_api_burst"
	input.Config.RecordRequestBodies = true
	input.Config.Scenarios[0].URL = target.URL
	input.Config.Scenarios[0].FieldRules = []generator.FieldRule{{Path: []string{"id"}, Regex: "ONE", Unique: true}}
	handler := newHandler(filepath.Join(t.TempDir(), "unique.jsonl"))
	if response := request(handler, "POST", "/api/preview", input); response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body)
	}
	waitForRun := func() (string, map[string]any) {
		t.Helper()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		deadline := time.NewTimer(2 * time.Second)
		defer deadline.Stop()
		for {
			select {
			case <-deadline.C:
				t.Fatal("run did not finish")
				return "", nil
			case <-ticker.C:
				response := request(handler, "GET", "/api/status", nil)
				var status struct {
					State  string         `json:"state"`
					Result map[string]any `json:"result"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
					t.Fatal(err)
				}
				if status.State != "running" && status.State != "stopping" {
					return status.State, status.Result
				}
			}
		}
	}
	for index := 0; index < 2; index++ {
		response := request(handler, "POST", "/api/run", input)
		if response.Code != http.StatusAccepted {
			t.Fatal(response.Code, response.Body)
		}
		state, result := waitForRun()
		if index == 0 {
			if state != "completed" {
				t.Fatalf("preview consumed a live value: %s %v", state, result)
			}
			record := result["responses"].([]any)[0].(map[string]any)
			if record["request_body"].(map[string]any)["id"] != "ONE" {
				t.Fatal(record)
			}
		} else if state != "failed" {
			t.Fatal("duplicate fresh run was accepted", state)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate body reached target", calls.Load())
	}
	input.Config.DataMode = "existing"
	input.Files["body.json"] = json.RawMessage(`{"id":"shared-record","label":""}`)
	if response := request(handler, "POST", "/api/run", input); response.Code != http.StatusAccepted {
		t.Fatal(response.Code, response.Body)
	}
	state, result := waitForRun()
	if state != "completed" || calls.Load() != 2 {
		t.Fatal(state, calls.Load())
	}
	if result["responses"].([]any)[0].(map[string]any)["request_body"].(map[string]any)["id"] != "shared-record" {
		t.Fatal(result)
	}
}

func TestPreviewUsesFieldRules(t *testing.T) {
	w := request(NewHandler(), "POST", "/api/preview", testSubmission())
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var result map[string]map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	id := result["create"]["id"].(string)
	if !regexp.MustCompile(`^PT-[0-9]{4}$`).MatchString(id) || result["create"]["label"] != id+"-ok" {
		t.Fatal(result)
	}
}

func TestConfigurableAPIPercentages(t *testing.T) {
	for _, test := range []struct {
		name        string
		percentages []int
		mode        string
		valid       bool
	}{
		{"single API", []int{100}, "", true},
		{"custom three API split", []int{30, 30, 40}, "weighted", true},
		{"ten APIs", []int{10, 10, 10, 10, 10, 10, 10, 10, 10, 10}, "weighted", true},
		{"under allocated", []int{30, 30}, "weighted", false},
		{"over allocated", []int{60, 60}, "weighted", false},
		{"zero percentage", []int{0, 100}, "weighted", false},
		{"negative percentage", []int{-10, 110}, "weighted", false},
		{"coordinated ignores percentages", []int{1, 1, 1}, "coordinated", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := testSubmission()
			scenario := input.Config.Scenarios[0]
			input.Config.ExecutionMode = test.mode
			input.Config.Scenarios = nil
			for index, percentage := range test.percentages {
				item := scenario
				item.Name = fmt.Sprintf("API %d", index+1)
				item.Weight = percentage
				input.Config.Scenarios = append(input.Config.Scenarios, item)
			}
			paths := []string{"/api/preview"}
			if !test.valid {
				paths = append(paths, "/api/run")
			}
			for _, path := range paths {
				response := request(NewHandler(), "POST", path, input)
				if test.valid && response.Code != http.StatusOK {
					t.Fatalf("%s: %d %s", path, response.Code, response.Body)
				}
				if !test.valid && response.Code != http.StatusBadRequest {
					t.Fatalf("%s: accepted invalid split: %d %s", path, response.Code, response.Body)
				}
			}
		})
	}
}

func TestUploadsCannotReadServerFiles(t *testing.T) {
	for _, kind := range []string{"path", "token", "placeholder", "expression", "null"} {
		t.Run(kind, func(t *testing.T) {
			input := testSubmission()
			switch kind {
			case "path":
				input.Config.Scenarios[0].BodyFile = "../README.md"
			case "token":
				input.Config.Scenarios[0].TokenEnv = "API_TOKEN"
			case "placeholder":
				input.Files["body.json"] = json.RawMessage(`{"id":"{{file_base64:secret}}","label":""}`)
			case "expression":
				input.Config.Scenarios[0].FieldRules[0] = generator.FieldRule{Path: []string{"id"}, Expression: `"{{file_" + "base64:secret}}"`}
			case "null":
				input.Files["body.json"] = json.RawMessage(`null`)
			}
			w := request(NewHandler(), "POST", "/api/preview", input)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		})
	}
}

func TestRejectsForeignOriginAndHost(t *testing.T) {
	for _, target := range []string{"http://localhost/api/status", "http://attacker.example/api/status"} {
		r := httptest.NewRequest("GET", target, nil)
		r.Header.Set("Origin", "https://attacker.example")
		w := httptest.NewRecorder()
		NewHandler().ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatal(w.Code)
		}
	}
}

func TestCoordinatedPreviewUsesSharedRecord(t *testing.T) {
	input := testSubmission()
	input.Config.ExecutionMode = "coordinated"
	input.Config.SharedDatasetFile = "shared.json"
	input.Files["shared.json"] = json.RawMessage(`[{"label":"shared value"}]`)
	input.Config.Scenarios[0].FieldRules = nil
	w := request(NewHandler(), "POST", "/api/preview", input)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "shared value") {
		t.Fatal(w.Code, w.Body)
	}
}

func TestRunStopAndResults(t *testing.T) {
	received := make(chan map[string]any, 100)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		received <- body
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"created":true}`))
	}))
	defer target.Close()
	input := testSubmission()
	input.Config.Duration = "5s"
	input.Config.Scenarios[0].URL = target.URL
	handler := NewHandler()
	if w := request(handler, "POST", "/api/run", input); w.Code != http.StatusAccepted {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(handler, "POST", "/api/run", input); w.Code != http.StatusConflict {
		t.Fatal(w.Code, w.Body)
	}
	select {
	case body := <-received:
		if body["label"] != body["id"].(string)+"-ok" {
			t.Fatal(body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not send request")
	}
	if w := request(handler, "POST", "/api/stop", nil); w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatal("run did not stop")
		case <-ticker.C:
			w := request(handler, "GET", "/api/status", nil)
			var status struct {
				State  string `json:"state"`
				Result struct {
					Summary struct {
						Completed int `json:"completed"`
					} `json:"summary"`
				} `json:"result"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
				t.Fatal(err)
			}
			if status.State == "stopped" {
				if status.Result.Summary.Completed < 1 {
					t.Fatal("missing results")
				}
				return
			}
		}
	}
}
