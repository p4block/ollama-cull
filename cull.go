package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Model struct {
	Name      string `json:"name"`
	Model     string `json:"model"`
	Size      int64  `json:"size"`
	ExpiresAt string `json:"expires_at"`
}

type PsResponse struct {
	Models []Model `json:"models"`
}

var (
	ollamaHost      string
	whitelist       map[string]bool
	pollInterval    time.Duration
	enableWorkHours bool
	workStart       time.Time
	workEnd         time.Time
	client          *http.Client
	signalChannel   chan os.Signal
)

func init() {
	// Load environment variables
	ollamaHost = mustGetEnv("OLLAMA_HOST", "http://localhost:11434")

	// Parse whitelist (comma-separated)
	whitelistStr := getEnv("WHITELIST_MODELS", "gemma3:12b")
	whitelist = make(map[string]bool)
	for _, model := range strings.Split(whitelistStr, ",") {
		model = strings.TrimSpace(model)
		if model != "" {
			whitelist[model] = true
		}
	}

	// Parse poll interval
	pollSeconds := mustGetInt("POLL_INTERVAL", 5)
	pollInterval = time.Duration(pollSeconds) * time.Second

	// Work hours configuration
	enableWorkHours = mustGetBool("ENABLE_WORK_HOURS", false)

	workStartStr := getEnv("WORK_HOURS_START", "09:00")
	workEndStr := getEnv("WORK_HOURS_END", "17:00")
	workStart = mustParseTime(workStartStr)
	workEnd = mustParseTime(workEndStr)
}

func mustGetEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	slog.Info("Using default", "key", key, "value", defaultVal)
	return defaultVal
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func mustGetInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		var parsed int
		fmt.Sscanf(val, "%d", &parsed)
		if parsed > 0 {
			return parsed
		}
	}
	return defaultVal
}

func mustGetBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		var parsed bool
		fmt.Sscanf(val, "%t", &parsed)
		if parsed {
			return parsed
		}
	}
	return defaultVal
}

func mustParseTime(timeStr string) time.Time {
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		slog.Error("Failed to parse time", "time", timeStr, "error", err)
		os.Exit(1)
	}
	return t
}

func isWorkHours() bool {
	now := time.Now()
	workStartToday := time.Date(now.Year(), now.Month(), now.Day(), workStart.Hour(), workStart.Minute(), 0, 0, now.Location())
	workEndToday := time.Date(now.Year(), now.Month(), now.Day(), workEnd.Hour(), workEnd.Minute(), 0, 0, now.Location())

	// Handle overnight work hours (e.g., 22:00 - 06:00)
	if workEnd.Before(workStart) {
		workEndToday = workEndToday.Add(24 * time.Hour)
	}

	return !now.Before(workStartToday) && !now.After(workEndToday)
}

func getRunningModels() ([]Model, error) {
	url := fmt.Sprintf("%s/api/ps", ollamaHost)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var response PsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Models, nil
}

func deleteModel(name string) error {
	url := fmt.Sprintf("%s/api/delete", ollamaHost)

	data := map[string]string{"model": name}
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Ollama delete API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("unexpected status from delete: %d", resp.StatusCode)
	}

	return nil
}

func runCuller() {
	slog.Info("Starting Ollama culler", "host", ollamaHost, "whitelist", whitelist)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			runCull()
		case <-signalChannel:
			slog.Info("Shutting down...")
			return
		}
	}
}

func runCull() {
	if enableWorkHours && isWorkHours() {
		slog.Debug("Skipping cull (work hours)")
		return
	}

	models, err := getRunningModels()
	if err != nil {
		slog.Error("Failed to get running models", "error", err)
		return
	}

	for _, model := range models {
		if whitelist[model.Name] {
			slog.Debug("Skipping whitelisted model", "model", model.Name)
			continue
		}

		slog.Info("Removing model", "model", model.Name)

		if err := deleteModel(model.Name); err != nil {
			slog.Warn("Failed to delete model", "model", model.Name, "error", err)
			continue
		}
	}
}

func main() {
	client = &http.Client{Timeout: 30 * time.Second}
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)

	runCuller()
}
