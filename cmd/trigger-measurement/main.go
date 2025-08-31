// trigger-measurement command is used to trigger a metric collection for a stations

package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

const DefaultPeriod = "P3D"
const DefaultTaskAPIPath = "/tasks/collectStationMeasurements"

type Config struct {
	APIEndpoint string
	APIKey      string
	TaskAPIPath string

	Period         string
	RequestTimeout time.Duration
}

func main() {
	fmt.Println("Triggering measurement...")
	config, err := loadConfigFromEnv()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Set defaults if not provided
	if config.TaskAPIPath == "" {
		config.TaskAPIPath = DefaultTaskAPIPath
	}
	if config.Period == "" {
		config.Period = DefaultPeriod
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 10 // default to 10 seconds
	}

	// TODO: collect stationID from args or pull from API
	stationID := "rhein-mannheim"
	if err := triggerMeasurementTask(config, stationID); err != nil {
		fmt.Printf("Error triggering measurement: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Measurement triggered successfully.")
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

	taskAPIPath := os.Getenv("WS_MEASUREMENT_TASK_PATH")
	if taskAPIPath == "" {
		taskAPIPath = DefaultTaskAPIPath
	}

	return &Config{
		APIEndpoint: apiEndpoint,
		APIKey:      apiKey,
		TaskAPIPath: taskAPIPath,
	}, nil
}

func triggerMeasurementTask(config *Config, stationID string) error {
	taskURL, err := url.JoinPath(config.APIEndpoint, config.TaskAPIPath)
	if err != nil {
		return fmt.Errorf("failed to construct task URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, taskURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	q := req.URL.Query()

	// Add required query parameters
	q.Add("station_id", stationID)
	q.Add("period", config.Period)
	req.URL.RawQuery = q.Encode()

	client := http.Client{
		Timeout: config.RequestTimeout * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func(resp *http.Response) {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %v\n", err)
		}
	}(resp)

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d - %s", resp.StatusCode, string(content))
	}

	return nil
}
