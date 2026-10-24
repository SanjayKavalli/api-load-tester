package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"go-api-load-tester/internal/config"
	"go-api-load-tester/internal/engine"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "configs/patient-import.json", "Path to load-test config")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, or error")
	logRequestBody := flag.Bool("log-request-body", true, "Log complete request bodies (may contain sensitive data)")
	logResponseBody := flag.Bool("log-response-body", true, "Log response bodies (may contain sensitive data)")
	responseDir := flag.String("response-dir", "responses", "Directory for the per-run response JSON file (empty disables recording)")
	maxResponseBody := flag.Int("max-response-body", 0, "Bytes of each response body to record (0 records complete bodies)")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	level, err := parseLogLevel(*logLevel)
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	slog.Info("loading configuration", "path", *configPath)
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	slog.Info("configuration loaded", "scenario_count", len(cfg.Scenarios))

	if err := engine.Run(cfg, *logRequestBody, *logResponseBody, *responseDir, *maxResponseBody); err != nil {
		return err
	}
	return nil
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level %q (use debug, info, warn, or error)", value)
	}
}
