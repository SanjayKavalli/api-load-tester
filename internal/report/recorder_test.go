package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRecorderWritesValidJSONFile(t *testing.T) {
	dir := t.TempDir()
	started := time.Date(2026, 10, 24, 10, 45, 30, 0, time.UTC)

	recorder, err := NewRecorder(dir, started, 0)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	if filepath.Base(recorder.Path()) != "responses_20261024-104530.json" {
		t.Errorf("Path() = %q, want a timestamped responses file", recorder.Path())
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recorder.Add(ResponseRecord{
				Scenario: "new_patient_import",
				Method:   "POST",
				Status:   200,
				Expected: true,
			}, []byte(`{"is_error":false}`))
		}()
	}
	wg.Wait()

	recorder.Add(ResponseRecord{Scenario: "blocked", Method: "POST", Status: 403}, []byte("<html>403</html>"))

	summary := RunSummary{Completed: 21, Statuses: map[int]int{200: 20, 403: 1}}
	if err := recorder.Close(summary); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(recorder.Path())
	if err != nil {
		t.Fatalf("read response file: %v", err)
	}

	var file struct {
		StartedAt     string           `json:"started_at"`
		Responses     []ResponseRecord `json:"responses"`
		ResponseCount int              `json:"response_count"`
		Summary       RunSummary       `json:"summary"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("response file is not valid JSON: %v", err)
	}
	if len(file.Responses) != 21 || file.ResponseCount != 21 {
		t.Fatalf("recorded %d responses (count field %d), want 21", len(file.Responses), file.ResponseCount)
	}
	if file.Summary.Statuses[200] != 20 {
		t.Errorf("summary statuses = %v, want 20 responses with status 200", file.Summary.Statuses)
	}

	seen := make(map[int]bool, len(file.Responses))
	for _, record := range file.Responses {
		if seen[record.Index] {
			t.Fatalf("duplicate response index %d", record.Index)
		}
		seen[record.Index] = true
	}

	var jsonBodies, textBodies int
	for _, record := range file.Responses {
		if len(record.Body) > 0 {
			jsonBodies++
		}
		if record.BodyText != "" {
			textBodies++
		}
	}
	if jsonBodies != 20 || textBodies != 1 {
		t.Errorf("json bodies = %d and text bodies = %d, want 20 and 1", jsonBodies, textBodies)
	}
}

func TestRecorderRecordsCompleteBodiesByDefault(t *testing.T) {
	recorder, err := NewRecorder(t.TempDir(), time.Now(), 0)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	largeText := []byte(strings.Repeat("a", 512<<10))
	largeJSON := []byte(`{"note":"` + strings.Repeat("b", 512<<10) + `"}`)
	recorder.Add(ResponseRecord{Scenario: "large_text", Status: 200}, largeText)
	recorder.Add(ResponseRecord{Scenario: "large_json", Status: 200}, largeJSON)
	if err := recorder.Close(RunSummary{}); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(recorder.Path())
	if err != nil {
		t.Fatalf("read response file: %v", err)
	}
	var file struct {
		Responses []ResponseRecord `json:"responses"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("response file is not valid JSON: %v", err)
	}
	if len(file.Responses) != 2 {
		t.Fatalf("recorded %d responses, want 2", len(file.Responses))
	}

	text := file.Responses[0]
	if text.Truncated {
		t.Error("default recorder truncated a large text body")
	}
	if text.BodyText != string(largeText) || text.BodyBytes != len(largeText) {
		t.Errorf("stored text body length = %d, want %d", len(text.BodyText), len(largeText))
	}

	jsonRecord := file.Responses[1]
	if jsonRecord.Truncated {
		t.Error("default recorder truncated a large JSON body")
	}
	if string(jsonRecord.Body) != string(largeJSON) {
		t.Errorf("stored JSON body length = %d, want %d", len(jsonRecord.Body), len(largeJSON))
	}
	if jsonRecord.BodyBytes != len(largeJSON) {
		t.Errorf("body_bytes = %d, want %d", jsonRecord.BodyBytes, len(largeJSON))
	}
}

func TestRecorderTruncatesOnlyWhenLimitIsSet(t *testing.T) {
	const limit = 64 << 10
	recorder, err := NewRecorder(t.TempDir(), time.Now(), limit)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	large := []byte(strings.Repeat("a", limit+100))
	small := []byte(`{"is_error":false}`)
	recorder.Add(ResponseRecord{Scenario: "large", Status: 200}, large)
	recorder.Add(ResponseRecord{Scenario: "small", Status: 200}, small)
	if err := recorder.Close(RunSummary{}); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(recorder.Path())
	if err != nil {
		t.Fatalf("read response file: %v", err)
	}
	var file struct {
		Responses []ResponseRecord `json:"responses"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("response file is not valid JSON: %v", err)
	}

	record := file.Responses[0]
	if !record.Truncated {
		t.Error("large body was not marked as truncated")
	}
	if record.BodyBytes != len(large) {
		t.Errorf("body_bytes = %d, want %d", record.BodyBytes, len(large))
	}
	if len(record.BodyText) != limit {
		t.Errorf("stored body length = %d, want %d", len(record.BodyText), limit)
	}

	if file.Responses[1].Truncated {
		t.Error("body under the limit was marked as truncated")
	}
	if string(file.Responses[1].Body) != string(small) {
		t.Errorf("small body = %s, want %s", file.Responses[1].Body, small)
	}
}

func TestNilRecorderIsSafe(t *testing.T) {
	var recorder *Recorder
	recorder.Add(ResponseRecord{Scenario: "none"}, []byte("{}"))
	if err := recorder.Close(RunSummary{}); err != nil {
		t.Fatalf("Close on nil recorder: %v", err)
	}
	if recorder.Path() != "" || recorder.Count() != 0 {
		t.Error("nil recorder should report an empty path and zero count")
	}
}
