package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
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
	return RunWithBodyLimits(cfg, logRequestBody, logResponseBody, responseDir, maxResponseBodyBytes, transport.DefaultMaxResponseBodyBytes)
}

func RunWithBodyLimits(cfg *config.Config, logRequestBody, logResponseBody bool, responseDir string, maxRecordedResponseBodyBytes, maxCapturedResponseBodyBytes int) error {
	return RunContextWithBodyLimits(context.Background(), cfg, logRequestBody, logResponseBody, responseDir, maxRecordedResponseBodyBytes, maxCapturedResponseBodyBytes)
}

func RunContext(ctx context.Context, cfg *config.Config, logRequestBody, logResponseBody bool, responseDir string, maxResponseBodyBytes int) error {
	return RunContextWithBodyLimits(ctx, cfg, logRequestBody, logResponseBody, responseDir, maxResponseBodyBytes, transport.DefaultMaxResponseBodyBytes)
}

func RunContextWithBodyLimits(ctx context.Context, cfg *config.Config, logRequestBody, logResponseBody bool, responseDir string, maxRecordedResponseBodyBytes, maxCapturedResponseBodyBytes int) error {
	if maxCapturedResponseBodyBytes < 0 {
		return fmt.Errorf("max captured response body bytes must be non-negative")
	}
	multiBurst := cfg.ExecutionMode == "multi_api_burst"
	if multiBurst && (len(cfg.Scenarios) == 0 || cfg.BurstSize > cfg.MaxInFlight/len(cfg.Scenarios)) {
		return fmt.Errorf("max_inflight must cover the complete multi-API burst")
	}
	duration, err := time.ParseDuration(cfg.Duration)
	if err != nil {
		return fmt.Errorf("parse load-test duration: %w", err)
	}
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return fmt.Errorf("parse request timeout: %w", err)
	}
	interval := time.Second / time.Duration(cfg.Rate)
	if multiBurst {
		interval = time.Second
		if cfg.BurstInterval != "" {
			interval, err = time.ParseDuration(cfg.BurstInterval)
			if err != nil || interval < time.Millisecond {
				return fmt.Errorf("invalid burst interval")
			}
		}
	}
	var store *generator.UniqueStore
	if cfg.DataMode != "existing" {
		needsHistory := false
		for _, scenario := range cfg.Scenarios {
			for _, rule := range scenario.FieldRules {
				needsHistory = needsHistory || rule.Unique
			}
		}
		if needsHistory {
			path := cfg.UniqueHistoryFile
			if path == "" {
				path, err = generator.DefaultHistoryPath()
				if err != nil {
					return err
				}
			}
			store, err = generator.OpenUniqueStore(path)
			if err != nil {
				return err
			}
			defer store.Close()
		}
	}
	session, err := generator.NewSession(store, cfg.DataMode == "existing")
	if err != nil {
		return err
	}
	globalToken := os.Getenv("API_TOKEN")
	if cfg.DisableGlobalToken {
		globalToken = ""
	}
	client := transport.NewWithResponseLimit(timeout, cfg.MaxInFlight, globalToken, maxCapturedResponseBodyBytes)

	var recorder *report.Recorder
	if strings.TrimSpace(responseDir) != "" {
		recorder, err = report.NewRecorder(responseDir, time.Now(), maxRecordedResponseBodyBytes)
		if err != nil {
			return err
		}
		slog.Info("recording responses", "file", recorder.Path(), "max_recorded_response_body_bytes", maxRecordedResponseBodyBytes, "max_captured_response_body_bytes", maxCapturedResponseBodyBytes)
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
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var arrivals <-chan time.Time = ticker.C
	if multiBurst {
		immediate := make(chan time.Time, 1)
		immediate <- time.Now()
		arrivals = immediate
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()

	var dropped, bodyErrors int
	var scheduledCount, generatedCount int
	var sentCount atomic.Int64
	sharedDatasetIndex := 0
	start := time.Now()
	running := true
	var runErr error

	for running {
		select {
		case <-ctx.Done():
			running = false
		case <-timer.C:
			running = false
		case <-arrivals:
			if multiBurst {
				arrivals = ticker.C
				if cfg.BurstMode != "repeat" {
					running = false
				}
			}
			scheduledRequests := scheduleBurst(cfg, totalWeight, &sharedDatasetIndex)
			scheduledCount += len(scheduledRequests)
			var preparedBodies [][]byte
			if multiBurst {
				if len(semaphore)+len(scheduledRequests) > cfg.MaxInFlight {
					dropped += len(scheduledRequests)
					continue
				}
				preparedBodies = make([][]byte, len(scheduledRequests))
				for index, scheduled := range scheduledRequests {
					if ctx.Err() != nil {
						running = false
						break
					}
					scenario := scheduled.scenario
					preparedBodies[index], err = generator.BuildBodyWithSession(scenario.BodyTemplate, scheduled.record, scenario.OverlayDatasetKeys, cfg.BaseDir, scenario.FieldRules, session)
					if err != nil {
						bodyErrors++
						runErr = fmt.Errorf("prepare synchronized burst: %w", err)
						running = false
						break
					}
					generatedCount++
				}
				if runErr != nil || ctx.Err() != nil {
					break
				}
			}
			startGate := make(chan struct{})
			for index, scheduled := range scheduledRequests {
				if ctx.Err() != nil {
					break
				}
				// Don't accumulate an unbounded queue if the service is saturated.
				select {
				case semaphore <- struct{}{}:
				default:
					dropped++
					continue
				}

				s := scheduled.scenario
				record := scheduled.record
				var body []byte
				if multiBurst {
					body = preparedBodies[index]
				} else {
					body, err = generator.BuildBodyWithSession(s.BodyTemplate, record, s.OverlayDatasetKeys, cfg.BaseDir, s.FieldRules, session)
				}
				if err != nil {
					bodyErrors++
					<-semaphore
					slog.Error("request body generation failed", "scenario", s.Name, "error", err)
					if errors.Is(err, generator.ErrUniqueGeneration) {
						runErr = err
						running = false
						break
					}
					continue
				}
				if !multiBurst {
					generatedCount++
				}

				wg.Add(1)
				go func(s config.Scenario, body []byte, start <-chan struct{}) {
					defer wg.Done()
					defer func() { <-semaphore }()

					<-start
					if ctx.Err() != nil {
						return
					}
					if logRequestBody {
						slog.Info("HTTP request body",
							"scenario", s.Name,
							"method", s.Method,
							"body_bytes", len(body),
							"request_body", string(body),
						)
					}
					requestStart := time.Now()
					sentCount.Add(1)
					resp := client.DoContext(ctx, s.Method, s.URL, s.Headers, body)
					expected := expectedStatus(resp.StatusCode, s.ExpectedStatuses, resp.Err)
					var requestBody json.RawMessage
					if cfg.RecordRequestBodies {
						requestBody = body
					}
					recorder.Add(report.ResponseRecord{
						RunID:             session.RunID,
						RequestBody:       requestBody,
						Scenario:          s.Name,
						Method:            s.Method,
						URL:               s.URL,
						StartedAt:         requestStart,
						DurationMs:        float64(resp.Duration.Microseconds()) / 1000,
						Status:            resp.StatusCode,
						ContentType:       resp.ContentType,
						Expected:          expected,
						Error:             errorText(resp.Err),
						Truncated:         resp.Truncated,
						DeclaredBodyBytes: resp.DeclaredBodyBytes,
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
			if multiBurst {
				ticker.Reset(interval)
			}
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

	if bodyErrors > 0 && runErr == nil {
		runErr = fmt.Errorf("run had %d body generation errors; %d requests completed", bodyErrors, allStats.Count)
	}
	if recorder != nil {
		perScenario := make(map[string]report.ScenarioSummary)
		for name, stats := range scenarioStats {
			perScenario[name] = stats.Snapshot()
		}
		summary := report.RunSummary{
			Outcome:              "passed",
			Scheduled:            scheduledCount,
			Generated:            generatedCount,
			Sent:                 sentCount.Load(),
			RunID:                session.RunID,
			Scenarios:            perScenario,
			ElapsedMs:            float64(elapsed.Microseconds()) / 1000,
			Completed:            allStats.Count,
			Failed:               allStats.Failed,
			NetworkErrors:        allStats.Network,
			DroppedArrivals:      dropped,
			BodyGenerationErrors: bodyErrors,
			Statuses:             allStats.Statuses,
		}
		if allStats.Failed > 0 || runErr != nil {
			summary.Outcome = "failed"
		}
		if bodyErrors > 0 && allStats.Count > 0 {
			summary.Outcome = "partial"
		}
		if ctx.Err() != nil {
			summary.Outcome = "stopped"
		}
		if err := recorder.Close(summary); err != nil {
			return err
		}
		fmt.Println("Responses written to:", recorder.Path())
		slog.Info("responses written", "file", recorder.Path(), "response_count", recorder.Count())
	}
	return runErr
}

func requestsPerBurst(cfg *config.Config) int {
	if cfg.ExecutionMode == "coordinated" || cfg.ExecutionMode == "multi_api_burst" {
		return cfg.BurstSize * len(cfg.Scenarios)
	}
	return cfg.BurstSize
}

func scheduleBurst(cfg *config.Config, totalWeight int, sharedDatasetIndex *int) []scheduledRequest {
	requests := make([]scheduledRequest, 0, requestsPerBurst(cfg))
	if cfg.ExecutionMode == "coordinated" || cfg.ExecutionMode == "multi_api_burst" {
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
			record = chooseRecord(scenario.Dataset, scenario.DatasetMode, &scenario.DatasetIndex)
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

func chooseRecord(records []map[string]any, mode string, index *int) map[string]any {
	if len(records) == 0 {
		return nil
	}
	if mode == "cycle" {
		return chooseSharedRecord(records, "cycle", index)
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
