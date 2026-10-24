package engine

import (
	"testing"

	"go-api-load-tester/internal/config"
)

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
