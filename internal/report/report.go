package report

import (
	"fmt"
	"sort"
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
)

type Item struct {
	Scenario string
	Status   int
	Duration time.Duration
	Err      error
	Expected bool
}

type Stats struct {
	Count               int
	Failed              int
	Network             int
	TotalDurationMicros uint64
	latencies           *hdrhistogram.Histogram
	Statuses            map[int]int
}

const maxTrackedLatencyMicros int64 = 365 * 24 * 60 * 60 * 1_000_000

type ScenarioSummary struct {
	Completed     int     `json:"completed"`
	Failed        int     `json:"failed"`
	NetworkErrors int     `json:"network_errors"`
	P95Ms         float64 `json:"p95_ms"`
	P99Ms         float64 `json:"p99_ms"`
}

func (s *Stats) Snapshot() ScenarioSummary {
	result := ScenarioSummary{Completed: s.Count, Failed: s.Failed, NetworkErrors: s.Network}
	if s.Count > 0 {
		result.P95Ms = float64(s.latencies.ValueAtQuantile(95)) / 1000
		result.P99Ms = float64(s.latencies.ValueAtQuantile(99)) / 1000
	}
	return result
}

func NewStats() *Stats {
	return &Stats{
		Statuses:  make(map[int]int),
		latencies: hdrhistogram.New(1, maxTrackedLatencyMicros, 3),
	}
}

func (s *Stats) Add(item Item) {
	s.Count++
	durationMicros := item.Duration.Microseconds()
	if durationMicros < 1 {
		durationMicros = 1
	}
	if durationMicros > maxTrackedLatencyMicros {
		durationMicros = maxTrackedLatencyMicros
	}
	_ = s.latencies.RecordValue(durationMicros)
	if item.Duration > 0 {
		s.TotalDurationMicros += uint64(item.Duration.Microseconds())
	}
	if item.Err != nil {
		s.Failed++
		s.Network++
		return
	}
	s.Statuses[item.Status]++
	if !item.Expected {
		s.Failed++
	}
}

func (s *Stats) Print(title string) {
	fmt.Printf("\n--- %s ---\n", title)
	fmt.Println("Completed:", s.Count)
	fmt.Println("Failed:", s.Failed)
	fmt.Println("Network/request errors:", s.Network)
	if s.Count > 0 {
		fmt.Printf("Average latency: %.2f ms\n", float64(s.TotalDurationMicros)/1000/float64(s.Count))
		fmt.Printf("P95 latency: %.2f ms\n", float64(s.latencies.ValueAtQuantile(95))/1000)
		fmt.Printf("P99 latency: %.2f ms\n", float64(s.latencies.ValueAtQuantile(99))/1000)
	}
	fmt.Println("HTTP statuses:")
	codes := make([]int, 0, len(s.Statuses))
	for code := range s.Statuses {
		codes = append(codes, code)
	}
	sort.Ints(codes)
	if len(codes) == 0 {
		fmt.Println("  (none)")
	}
	for _, code := range codes {
		fmt.Printf("  %d: %d\n", code, s.Statuses[code])
	}
}
