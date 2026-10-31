package generator

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
)

func TestUniqueValuesSurviveRepeatedRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	rule := []FieldRule{{Path: []string{"id"}, Regex: "[AB]", Unique: true}}
	seen := map[string]bool{}
	for run := 0; run < 2; run++ {
		store, err := OpenUniqueStore(path)
		if err != nil {
			t.Fatal(err)
		}
		session, _ := NewSession(store, false)
		body, err := BuildBodyWithSession(map[string]any{"id": ""}, nil, false, "", rule, session)
		store.Close()
		if err != nil {
			t.Fatal(err)
		}
		if seen[string(body)] {
			t.Fatal("reused a previous run's value")
		}
		seen[string(body)] = true
	}
	store, err := OpenUniqueStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	session, _ := NewSession(store, false)
	if _, err := BuildBodyWithSession(map[string]any{"id": ""}, nil, false, "", rule, session); !errors.Is(err, ErrUniqueUnavailable) {
		t.Fatalf("want exhaustion error, got %v", err)
	}
}

func TestRunIdentityAndExistingRecords(t *testing.T) {
	first, _ := NewSession(nil, false)
	second, _ := NewSession(nil, false)
	if first.RunID == second.RunID {
		t.Fatal("run IDs repeated")
	}
	rules := []FieldRule{{Path: []string{"id"}, Expression: `run_id + "-" + string(sequence)`, Unique: true}}
	for index := 0; index < 3; index++ {
		body, err := BuildBodyWithSession(map[string]any{"id": ""}, nil, false, "", rules, first)
		if err != nil || !json.Valid(body) {
			t.Fatal(string(body), err)
		}
	}
	existing, _ := NewSession(nil, true)
	body, err := BuildBodyWithSession(map[string]any{"id": ""}, map[string]any{"id": "shared-record"}, true, "", rules, existing)
	if err != nil || string(body) != `{"id":"shared-record"}` {
		t.Fatal(string(body), err)
	}
}

func TestUniqueHistoryRejectsConcurrentRunners(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	first, err := OpenUniqueStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := OpenUniqueStore(path)
	if err == nil {
		second.Close()
		t.Fatal("a second runner acquired the active uniqueness history")
	}
}
