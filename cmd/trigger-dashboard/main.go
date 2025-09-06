package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

const DefaultTaskAPIPath = "/tasks/buildDashboard"

type Config struct {
	APIEndpoint string
	APIKey      string
	TaskAPIPath string
	TaskTimeout int

	RequestTimeout int
}

func main() {
	fmt.Println("Triggering dashboard build...")
	config, err := loadConfigFromEnv()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	stationID := "rhein-mannheim"
	if err := triggerDashboardBuild(config, stationID); err != nil {
		fmt.Printf("Error triggering dashboard build: %v\n", err)
		return
	}

	fmt.Println("Dashboard build triggered successfully.")
}

func loadConfigFromEnv() (*Config, error) {
	apiEndpoint := os.Getenv("WS_API_ENDPOINT")
	if apiEndpoint == "" {
		return nil, fmt.Errorf("WS_API_ENDPOINT is not set")
	}

	apiKey := os.Getenv("WS_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("WS_API_KEY is not set")
	}

	taskAPIPath := os.Getenv("WS_DASHBOARD_TASK_PATH")
	if taskAPIPath == "" {
		taskAPIPath = "/tasks/buildDashboard"
	}

	return &Config{
		APIEndpoint:    apiEndpoint,
		APIKey:         apiKey,
		TaskAPIPath:    taskAPIPath,
		TaskTimeout:    10,
		RequestTimeout: 10,
	}, nil
}

func triggerDashboardBuild(config *Config, stationID string) error {
	// Dummy implementation for illustration purposes
	fmt.Printf("Triggering dashboard build for %s at %s%s\n", stationID, config.APIEndpoint, config.TaskAPIPath)

	httpClient := &http.Client{
		Timeout: time.Duration(config.RequestTimeout) * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.TaskTimeout)*time.Second)
	defer cancel()

	resourceURL, err := url.JoinPath(config.APIEndpoint, config.TaskAPIPath)
	if err != nil {
		return fmt.Errorf("failed to join URL path: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resourceURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Accept", "application/json")

	q := req.URL.Query()
	q.Add("station_id", stationID)
	q.Add("language_code", "en")
	q.Add("timezone", "utc")
	req.URL.RawQuery = q.Encode()

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform HTTP request: %w", err)
	}
	defer func(resp *http.Response) {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %v\n", err)
		}
	}(resp)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned non-200/202 status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
