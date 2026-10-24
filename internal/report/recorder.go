package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ResponseRecord is one captured HTTP response.
type ResponseRecord struct {
	Index       int             `json:"index"`
	Scenario    string          `json:"scenario"`
	Method      string          `json:"method"`
	URL         string          `json:"url"`
	StartedAt   time.Time       `json:"started_at"`
	DurationMs  float64         `json:"duration_ms"`
	Status      int             `json:"status"`
	ContentType string          `json:"content_type,omitempty"`
	Expected    bool            `json:"expected"`
	Error       string          `json:"error,omitempty"`
	Body        json.RawMessage `json:"body,omitempty"`
	BodyText    string          `json:"body_text,omitempty"`
	BodyBytes   int             `json:"body_bytes"`
	Truncated   bool            `json:"body_truncated,omitempty"`
}

// RunSummary is the aggregate written at the end of a response file.
type RunSummary struct {
	ElapsedMs            float64     `json:"elapsed_ms"`
	Completed            int         `json:"completed"`
	Failed               int         `json:"failed"`
	NetworkErrors        int         `json:"network_errors"`
	DroppedArrivals      int         `json:"dropped_arrivals"`
	BodyGenerationErrors int         `json:"body_generation_errors"`
	Statuses             map[int]int `json:"statuses"`
}

// Recorder streams responses into a per-run JSON file. It is safe for
// concurrent use by the request goroutines.
type Recorder struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	count    int
	maxBody  int
	writeErr error
	closed   bool
}

// NewRecorder creates dir if needed and opens responses_<timestamp>.json.
//
// maxBodyBytes caps how much of each response body is stored. Use zero or a
// negative value to record every response body in full, which is the default.
func NewRecorder(dir string, startedAt time.Time, maxBodyBytes int) (*Recorder, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create response directory: %w", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("responses_%s.json", startedAt.Format("20060102-150405")))
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create response file: %w", err)
	}

	r := &Recorder{file: file, path: path, maxBody: maxBodyBytes}
	header := fmt.Sprintf("{\n  \"started_at\": %q,\n  \"responses\": [\n", startedAt.Format(time.RFC3339Nano))
	if _, err := file.WriteString(header); err != nil {
		file.Close()
		return nil, fmt.Errorf("write response file header: %w", err)
	}
	return r, nil
}

// Path returns the file responses are written to.
func (r *Recorder) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

// Count returns how many responses have been recorded.
func (r *Recorder) Count() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

// Add appends one response. Body is stored as JSON when it parses as JSON and
// as text otherwise. Bodies are recorded in full unless the recorder was built
// with a positive body limit. A nil Recorder discards the record.
func (r *Recorder) Add(record ResponseRecord, body []byte) {
	if r == nil {
		return
	}

	record.BodyBytes = len(body)
	if r.maxBody > 0 && len(body) > r.maxBody {
		body = body[:r.maxBody]
		record.Truncated = true
	}
	if len(body) > 0 {
		if !record.Truncated && json.Valid(body) {
			record.Body = json.RawMessage(body)
		} else {
			record.BodyText = string(body)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.writeErr != nil {
		return
	}

	record.Index = r.count + 1
	encoded, err := json.Marshal(record)
	if err != nil {
		r.writeErr = fmt.Errorf("encode response record: %w", err)
		return
	}

	separator := ",\n    "
	if r.count == 0 {
		separator = "    "
	}
	if _, err := r.file.WriteString(separator + string(encoded)); err != nil {
		r.writeErr = fmt.Errorf("write response record: %w", err)
		return
	}
	r.count++
}

// Close writes the run summary and closes the file.
func (r *Recorder) Close(summary any) error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return r.writeErr
	}
	r.closed = true

	encodedSummary, err := json.MarshalIndent(summary, "  ", "  ")
	if err != nil {
		r.file.Close()
		return fmt.Errorf("encode response summary: %w", err)
	}

	footer := fmt.Sprintf("\n  ],\n  \"response_count\": %d,\n  \"summary\": %s\n}\n", r.count, encodedSummary)
	if _, err := r.file.WriteString(footer); err != nil {
		r.file.Close()
		return fmt.Errorf("write response summary: %w", err)
	}
	if err := r.file.Close(); err != nil {
		return fmt.Errorf("close response file: %w", err)
	}
	return r.writeErr
}
