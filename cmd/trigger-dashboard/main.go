package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/timgluz/wasserspiegel/dashboard"
	"github.com/timgluz/wasserspiegel/task"
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

	if err := triggerAllDashboardBuilds(config); err != nil {
		fmt.Printf("Error triggering dashboard builds: %v\n", err)
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
		taskAPIPath = DefaultTaskAPIPath
	}

	return &Config{
		APIEndpoint:    apiEndpoint,
		APIKey:         apiKey,
		TaskAPIPath:    taskAPIPath,
		TaskTimeout:    10,
		RequestTimeout: 10,
	}, nil
}

func triggerAllDashboardBuilds(config *Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.RequestTimeout)*time.Second)
	defer cancel()

	httpClient := &http.Client{
		Timeout: time.Duration(config.RequestTimeout) * time.Second,
	}

	dashboardRepository := dashboard.NewAPIRepository(httpClient, config.APIEndpoint, config.APIKey)
	dashboardCh, errCh := dashboard.StreamDashboards(ctx, dashboardRepository, 0, 100)
	if dashboardCh == nil && errCh == nil {
		return fmt.Errorf("no dashboards found")
	}

	for {
		select {
		case dashboard, ok := <-dashboardCh:
			if !ok {
				dashboardCh = nil
				continue
			}

			fmt.Printf("Processing dashboard ID: %s, StationID: %s\n", dashboard.ID, dashboard.StationID)

			builderOptions, ok := mapToDashboardBuilderOptions(dashboard)
			if !ok {
				fmt.Printf("Skipping dashboard %s due to missing builder options\n", dashboard.ID)
				continue
			}

			if err := triggerDashboardBuild(config, builderOptions); err != nil {
				fmt.Printf("Error triggering dashboard build for %s: %v\n", dashboard.ID, err)
				continue
			}

		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			return fmt.Errorf("error iterating dashboards: %v", err)
		case <-ctx.Done():
			fmt.Println("context cancelled, stopping iteration")

			return nil
		default:
			// No operation, just to avoid blocking
			time.Sleep(100 * time.Millisecond)
		}

		if dashboardCh == nil && errCh == nil {
			break
		}
	}

	fmt.Println("Completed processing all dashboards.")

	return nil
}

func mapToDashboardBuilderOptions(dashboardItem dashboard.ListItem) (task.DashboardBuilderOptions, bool) {
	return task.DashboardBuilderOptions{
		StationID:    dashboardItem.StationID,
		LanguageCode: dashboardItem.LanguageCode,
		Timezone:     dashboardItem.Timezone,
	}, true
}

func triggerDashboardBuild(config *Config, opts task.DashboardBuilderOptions) error {
	stationID := opts.StationID
	if stationID == "" {
		return fmt.Errorf("empty station ID")
	}

	fmt.Printf("Triggering dashboard build for %s at %s%s\n", stationID, config.APIEndpoint, config.TaskAPIPath)

	httpClient := &http.Client{
		Timeout: time.Duration(config.TaskTimeout) * time.Second,
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
	q.Add("language_code", opts.LanguageCode)
	q.Add("timezone", opts.Timezone)
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
