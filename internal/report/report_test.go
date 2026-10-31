package report

import (
	"math"
	"testing"
	"time"
)

func TestStatsUseBoundedLatencyHistogram(t *testing.T) {
	stats := NewStats()
	storageBytes := stats.latencies.ByteSize()

	for sample := 0; sample < 100_000; sample++ {
		stats.Add(Item{
			Duration: time.Duration(sample%100+1) * time.Millisecond,
			Expected: true,
		})
	}

	if got := stats.latencies.ByteSize(); got != storageBytes {
		t.Fatalf("histogram storage grew from %d to %d bytes", storageBytes, got)
	}
	if got := stats.latencies.TotalCount(); got != 100_000 {
		t.Fatalf("histogram recorded %d samples, want 100000", got)
	}

	summary := stats.Snapshot()
	if summary.Completed != 100_000 {
		t.Errorf("completed = %d, want 100000", summary.Completed)
	}
	if math.Abs(summary.P95Ms-95) > 0.2 {
		t.Errorf("p95 = %.3f ms, want approximately 95 ms", summary.P95Ms)
	}
	if math.Abs(summary.P99Ms-99) > 0.2 {
		t.Errorf("p99 = %.3f ms, want approximately 99 ms", summary.P99Ms)
	}
	averageMs := float64(stats.TotalDurationMicros) / 1000 / float64(stats.Count)
	if math.Abs(averageMs-50.5) > 0.01 {
		t.Errorf("average = %.3f ms, want 50.5 ms", averageMs)
	}
}
