// trigger-measurement command is used to trigger a metric collection for a stations

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/timgluz/wasserspiegel/station"
)

const DefaultPeriod = "P3D"
const DefaultTaskAPIPath = "/tasks/collectStationMeasurements"

type Config struct {
	APIEndpoint string
	APIKey      string
	TaskAPIPath string

	Period         string
	RequestTimeout time.Duration
	TaskTimeout    time.Duration
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

	startTime := time.Now()
	if err := triggerAllStationsMeasurement(config); err != nil {
		fmt.Printf("Error triggering measurements for all stations: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Measurement collection took %f seconds.\n", time.Since(startTime).Seconds())
}

func triggerAllStationsMeasurement(config *Config) error {
	httpClient := &http.Client{
		Timeout: config.RequestTimeout * time.Second,
	}

	stationRepository := station.NewAPIRepository(httpClient, config.APIEndpoint, config.APIKey)
	if !stationRepository.IsReady() {
		fmt.Println("Station repository is not ready.")
		os.Exit(1)
	}

	ctx := context.Background()
	stationCh, errCh := station.StreamStations(ctx, stationRepository, 0, 0)
	if stationCh == nil || errCh == nil {
		return fmt.Errorf("failed to initialize station iterator")
	}

	taskClient := &http.Client{
		Timeout: config.TaskTimeout * time.Second,
	}

	defer ctx.Done()
	var stationCount int
	for {
		select {
		case station, ok := <-stationCh:
			if !ok {
				fmt.Println("Completed iterating stations.")
				stationCh = nil
				continue
			}

			if station.IsDisabled {
				fmt.Printf("Station %s is disabled, skipping measurement trigger.\n", station.ID)
				continue
			}

			fmt.Printf("Found station: ID=%s, Name=%s\n", station.ID, station.Name)
			if err := triggerMeasurementTask(taskClient, config, station.ID); err != nil {
				fmt.Printf("Error triggering measurement for station %s: %v\n", station.ID, err)
				continue
			}
			stationCount++
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			return fmt.Errorf("error iterating stations: %v", err)
		case <-ctx.Done():
			fmt.Println("context cancelled, stopping iteration")

			return nil
		default:
			// No operation, just to avoid blocking
			time.Sleep(100 * time.Millisecond)
		}

		if stationCh == nil && errCh == nil {
			break
		}
	}

	if stationCount == 0 {
		fmt.Println("No stations found to trigger measurement.")
		return nil
	}

	fmt.Printf("Successfully triggered measurement for %d stations.\n", stationCount)
	return nil
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
		APIEndpoint:    apiEndpoint,
		APIKey:         apiKey,
		TaskAPIPath:    taskAPIPath,
		RequestTimeout: 10, // default to 10 seconds
		TaskTimeout:    30, // default to 30 seconds
		Period:         os.Getenv("WS_MEASUREMENT_PERIOD"),
	}, nil
}

func triggerMeasurementTask(client *http.Client, config *Config, stationID string) error {
	if client == nil {
		return fmt.Errorf("http client is required")
	}

	if stationID == "" {
		return fmt.Errorf("stationID is required")
	}

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
