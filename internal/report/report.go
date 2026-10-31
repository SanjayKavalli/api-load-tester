package report

import (
	"fmt"
	"sort"
	"time"
)

type Item struct {
	Scenario string
	Status   int
	Duration time.Duration
	Err      error
	Expected bool
}

type Stats struct {
	Count     int
	Failed    int
	Network   int
	Durations []time.Duration
	Statuses  map[int]int
}

type ScenarioSummary struct {
	Completed     int     `json:"completed"`
	Failed        int     `json:"failed"`
	NetworkErrors int     `json:"network_errors"`
	P95Ms         float64 `json:"p95_ms"`
	P99Ms         float64 `json:"p99_ms"`
}

func (s *Stats) Snapshot() ScenarioSummary {
	result := ScenarioSummary{Completed: s.Count, Failed: s.Failed, NetworkErrors: s.Network}
	if len(s.Durations) > 0 {
		result.P95Ms = percentile(s.Durations, 95)
		result.P99Ms = percentile(s.Durations, 99)
	}
	return result
}

func NewStats() *Stats {
	return &Stats{Statuses: make(map[int]int)}
}

func (s *Stats) Add(item Item) {
	s.Count++
	s.Durations = append(s.Durations, item.Duration)
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
	if len(s.Durations) > 0 {
		fmt.Printf("Average latency: %.2f ms\n", average(s.Durations))
		fmt.Printf("P95 latency: %.2f ms\n", percentile(s.Durations, 95))
		fmt.Printf("P99 latency: %.2f ms\n", percentile(s.Durations, 99))
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

func average(values []time.Duration) float64 {
	var total time.Duration
	for _, v := range values {
		total += v
	}
	return float64(total.Microseconds()) / 1000 / float64(len(values))
}

func percentile(values []time.Duration, p int) float64 {
	copyValues := append([]time.Duration(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	index := (p*len(copyValues)+99)/100 - 1
	if index < 0 {
		index = 0
	}
	if index >= len(copyValues) {
		index = len(copyValues) - 1
	}
	return float64(copyValues[index].Microseconds()) / 1000
}
