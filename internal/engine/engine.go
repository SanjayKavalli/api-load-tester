package engine

import (
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"go-api-load-tester/internal/config"
	"go-api-load-tester/internal/generator"
	"go-api-load-tester/internal/report"
	"go-api-load-tester/internal/transport"
)

type result struct {
	scenario string
	item     report.Item
}

type scheduledRequest struct {
	scenario *config.Scenario
	record   map[string]any
}

func Run(cfg *config.Config, logRequestBody, logResponseBody bool, responseDir string, maxResponseBodyBytes int) error {
	duration, err := time.ParseDuration(cfg.Duration)
	if err != nil {
		return fmt.Errorf("parse load-test duration: %w", err)
	}
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return fmt.Errorf("parse request timeout: %w", err)
	}
	client := transport.New(timeout, cfg.MaxInFlight, os.Getenv("API_TOKEN"))

	var recorder *report.Recorder
	if strings.TrimSpace(responseDir) != "" {
		recorder, err = report.NewRecorder(responseDir, time.Now(), maxResponseBodyBytes)
		if err != nil {
			return err
		}
		slog.Info("recording responses", "file", recorder.Path(), "max_response_body_bytes", maxResponseBodyBytes)
	}

	slog.Info("load test started",
		"duration", duration,
		"bursts_per_second", cfg.Rate,
		"execution_mode", cfg.ExecutionMode,
		"virtual_users_per_burst", cfg.BurstSize,
		"requests_per_burst", requestsPerBurst(cfg),
		"max_in_flight", cfg.MaxInFlight,
		"scenario_count", len(cfg.Scenarios),
	)
	slog.Info("run load tests only against an authorized test or staging target")
	for _, s := range cfg.Scenarios {
		slog.Info("load-test scenario configured",
			"scenario", s.Name,
			"weight", s.Weight,
			"method", s.Method,
		)
	}

	results := make(chan result, cfg.MaxInFlight)
	semaphore := make(chan struct{}, cfg.MaxInFlight)
	var wg sync.WaitGroup

	allStats := report.NewStats()
	scenarioStats := make(map[string]*report.Stats)
	for _, s := range cfg.Scenarios {
		scenarioStats[s.Name] = report.NewStats()
	}

	// The collector drains results while requests run, avoiding a full result channel.
	var collector sync.WaitGroup
	collector.Add(1)
	go func() {
		defer collector.Done()
		for r := range results {
			allStats.Add(r.item)
			scenarioStats[r.scenario].Add(r.item)
		}
	}()

	totalWeight := 0
	for _, s := range cfg.Scenarios {
		totalWeight += s.Weight
	}
	interval := time.Second / time.Duration(cfg.Rate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	timer := time.NewTimer(duration)
	defer timer.Stop()

	var dropped, bodyErrors int
	sharedDatasetIndex := 0
	start := time.Now()
	running := true

	for running {
		select {
		case <-timer.C:
			running = false
		case <-ticker.C:
			startGate := make(chan struct{})
			for _, scheduled := range scheduleBurst(cfg, totalWeight, &sharedDatasetIndex) {
				// Don't accumulate an unbounded queue if the service is saturated.
				select {
				case semaphore <- struct{}{}:
				default:
					dropped++
					continue
				}

				s := scheduled.scenario
				record := scheduled.record
				body, err := generator.BuildBody(s.BodyTemplate, record, s.OverlayDatasetKeys, cfg.BaseDir)
				if err != nil {
					bodyErrors++
					<-semaphore
					slog.Error("request body generation failed", "scenario", s.Name, "error", err)
					continue
				}

				wg.Add(1)
				go func(s config.Scenario, body []byte, start <-chan struct{}) {
					defer wg.Done()
					defer func() { <-semaphore }()

					<-start
					if logRequestBody {
						slog.Info("HTTP request body",
							"scenario", s.Name,
							"method", s.Method,
							"body_bytes", len(body),
							"request_body", string(body),
						)
					}
					requestStart := time.Now()
					resp := client.Do(s.Method, s.URL, s.Headers, body)
					expected := expectedStatus(resp.StatusCode, s.ExpectedStatuses, resp.Err)
					recorder.Add(report.ResponseRecord{
						Scenario:    s.Name,
						Method:      s.Method,
						URL:         s.URL,
						StartedAt:   requestStart,
						DurationMs:  float64(resp.Duration.Microseconds()) / 1000,
						Status:      resp.StatusCode,
						ContentType: resp.ContentType,
						Expected:    expected,
						Error:       errorText(resp.Err),
					}, resp.Body)
					responseAttrs := []any{
						"status", resp.StatusCode,
						"content_type", resp.ContentType,
						"duration", resp.Duration,
					}
					if logResponseBody {
						const maxLoggedResponseBody = 4096
						body := resp.Body
						truncated := len(body) > maxLoggedResponseBody
						if truncated {
							body = body[:maxLoggedResponseBody]
						}
						responseAttrs = append(responseAttrs,
							"response_body", string(body),
							"response_body_truncated", truncated,
						)
					}
					if resp.Err != nil {
						attrs := append([]any{
							"scenario", s.Name,
							"method", s.Method,
							"error", resp.Err,
						}, responseAttrs...)
						slog.Warn("HTTP request failed", attrs...)
					} else if !expected {
						attrs := append([]any{
							"scenario", s.Name,
							"method", s.Method,
						}, responseAttrs...)
						slog.Warn("HTTP response status was unexpected", attrs...)
					} else if logResponseBody {
						attrs := append([]any{
							"scenario", s.Name,
							"method", s.Method,
						}, responseAttrs...)
						slog.Info("HTTP response received", attrs...)
					} else {
						slog.Debug("HTTP request completed",
							"scenario", s.Name,
							"method", s.Method,
							"status", resp.StatusCode,
							"content_type", resp.ContentType,
							"duration", resp.Duration,
						)
					}
					results <- result{
						scenario: s.Name,
						item: report.Item{
							Scenario: s.Name,
							Status:   resp.StatusCode,
							Duration: resp.Duration,
							Err:      resp.Err,
							Expected: expected,
						},
					}
				}(*s, body, startGate)
			}
			close(startGate)
		}
	}

	wg.Wait()
	close(results)
	collector.Wait()
	elapsed := time.Since(start).Round(time.Millisecond)

	fmt.Println("\n========== LOAD TEST SUMMARY ==========")
	fmt.Println("Elapsed:", elapsed)
	fmt.Println("Dropped arrivals (max concurrency reached):", dropped)
	fmt.Println("Body generation errors:", bodyErrors)
	allStats.Print("ALL SCENARIOS")
	fmt.Println("\n========== PER-SCENARIO RESULTS ==========")
	for _, s := range cfg.Scenarios {
		scenarioStats[s.Name].Print(s.Name)
	}
	slog.Info("load test completed",
		"elapsed", elapsed,
		"completed", allStats.Count,
		"failed", allStats.Failed,
		"network_errors", allStats.Network,
		"dropped_arrivals", dropped,
		"body_generation_errors", bodyErrors,
	)

	if recorder != nil {
		summary := report.RunSummary{
			ElapsedMs:            float64(elapsed.Microseconds()) / 1000,
			Completed:            allStats.Count,
			Failed:               allStats.Failed,
			NetworkErrors:        allStats.Network,
			DroppedArrivals:      dropped,
			BodyGenerationErrors: bodyErrors,
			Statuses:             allStats.Statuses,
		}
		if err := recorder.Close(summary); err != nil {
			return err
		}
		fmt.Println("Responses written to:", recorder.Path())
		slog.Info("responses written", "file", recorder.Path(), "response_count", recorder.Count())
	}
	return nil
}

func requestsPerBurst(cfg *config.Config) int {
	if cfg.ExecutionMode == "coordinated" {
		return cfg.BurstSize * len(cfg.Scenarios)
	}
	return cfg.BurstSize
}

func scheduleBurst(cfg *config.Config, totalWeight int, sharedDatasetIndex *int) []scheduledRequest {
	requests := make([]scheduledRequest, 0, requestsPerBurst(cfg))
	if cfg.ExecutionMode == "coordinated" {
		for virtualUser := 0; virtualUser < cfg.BurstSize; virtualUser++ {
			record := chooseSharedRecord(cfg.SharedDataset, cfg.SharedDatasetMode, sharedDatasetIndex)
			for i := range cfg.Scenarios {
				requests = append(requests, scheduledRequest{
					scenario: &cfg.Scenarios[i],
					record:   record,
				})
			}
		}
		return requests
	}

	for virtualUser := 0; virtualUser < cfg.BurstSize; virtualUser++ {
		scenario := choose(cfg.Scenarios, totalWeight)
		var record map[string]any
		if len(scenario.Dataset) > 0 {
			record = chooseRecord(scenario.Dataset, scenario.DatasetMode)
		}
		requests = append(requests, scheduledRequest{scenario: scenario, record: record})
	}
	return requests
}

func chooseSharedRecord(records []map[string]any, mode string, index *int) map[string]any {
	if len(records) == 0 {
		return nil
	}
	switch mode {
	case "random":
		return records[rand.Intn(len(records))]
	case "cycle":
		record := records[*index%len(records)]
		*index++
		return record
	default:
		return records[0]
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func choose(scenarios []config.Scenario, totalWeight int) *config.Scenario {
	pick := rand.Intn(totalWeight)
	for i := range scenarios {
		pick -= scenarios[i].Weight
		if pick < 0 {
			return &scenarios[i]
		}
	}
	return &scenarios[len(scenarios)-1]
}

func chooseRecord(records []map[string]any, mode string) map[string]any {
	if len(records) == 0 {
		return nil
	}
	if mode == "cycle" {
		// This simple starter uses random selection for concurrent requests.
		// Random selection avoids shared mutable index state across goroutines.
	}
	return records[rand.Intn(len(records))]
}

func expectedStatus(status int, expected []int, err error) bool {
	if err != nil {
		return false
	}
	if len(expected) == 0 {
		return status >= 200 && status < 300
	}
	for _, code := range expected {
		if status == code {
			return true
		}
	}
	return false
}
