package station

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

var (
	ErrBaseURLNotSet    = fmt.Errorf("base URL not set")
	ErrHTTPClientNotSet = fmt.Errorf("HTTP client not set")
	ErrNoContent        = fmt.Errorf("no content available")
	ErrInvalidStationID = fmt.Errorf("invalid station ID provided")
	ErrUnmarshalFailed  = fmt.Errorf("failed to unmarshal content")
	ErrResourceNotFound = fmt.Errorf("resource not found")
)

type Provider interface {
	GetStations(ctx context.Context) (*StationCollection, error)
	GetStation(ctx context.Context, id string) (*Station, error)
	GetStationWaterLevel(ctx context.Context, id string) (*WaterLevelCollection, error)
	IsReady() bool
}

type HTTPProvider struct {
	baseURL string

	client *http.Client
	logger *slog.Logger
}

func NewHTTPProvider(baseURL string, client *http.Client, logger *slog.Logger) *HTTPProvider {
	return &HTTPProvider{
		baseURL: baseURL,
		client:  client,
		logger:  logger,
	}
}

func (p *HTTPProvider) IsReady() bool {
	if p.logger == nil {
		fmt.Println("Logger of HTTPProvider is not initialized")
		return false
	}

	if p.baseURL == "" {
		p.logger.Error("Base URL is not set for HTTPProvider")
		return false
	}

	if p.client == nil {
		p.logger.Error("HTTP client is not set for HTTPProvider")
		return false
	}

	p.logger.Info("HTTPProvider is ready", "baseURL", p.baseURL)
	return true
}

func (p *HTTPProvider) GetStations(ctx context.Context) (*StationCollection, error) {
	defer ctx.Done()

	if p.baseURL == "" {
		return nil, ErrBaseURLNotSet
	}

	if p.client == nil {
		return nil, ErrHTTPClientNotSet
	}

	resourceURL := p.baseURL + "/stations.json"
	req, err := p.client.Get(resourceURL)
	if err != nil {
		return nil, err
	}
	defer req.Body.Close()

	if req.StatusCode != http.StatusOK {
		if req.StatusCode == http.StatusNotFound {
			return nil, ErrResourceNotFound
		}

		return nil, fmt.Errorf("failed to fetch stations: %s", req.Status)
	}

	content, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read content from URL %s: %w", resourceURL, err)
	}

	if len(content) == 0 {
		p.logger.Warn("No content received from URL", "url", resourceURL)
		return nil, ErrNoContent
	}
	p.logger.Debug("Content successfully read from URL", "url", resourceURL, "length", len(content))

	stations := StationList{}
	if err := json.Unmarshal(content, &stations); err != nil {
		return nil, ErrUnmarshalFailed
	}

	if len(stations) == 0 {
		p.logger.Warn("No stations found in content, check if data schema has changed", "url", resourceURL)
		return nil, ErrNoContent
	}

	p.logger.Info("Successfully fetched stations", "count", len(stations))
	return &StationCollection{
		Stations: stations,
	}, nil
}

func (p *HTTPProvider) GetStation(ctx context.Context, id string) (*Station, error) {
	defer ctx.Done()

	if p.baseURL == "" {
		return nil, ErrBaseURLNotSet
	}

	if p.client == nil {
		return nil, ErrHTTPClientNotSet
	}

	resourceURL := fmt.Sprintf("%s/stations/%s.json", p.baseURL, id)
	req, err := p.client.Get(resourceURL)
	if err != nil {
		return nil, err
	}

	defer req.Body.Close()

	if req.StatusCode != http.StatusOK {
		if req.StatusCode == http.StatusNotFound {
			return nil, ErrResourceNotFound
		}

		return nil, fmt.Errorf("failed to fetch water level for station %s: %s", id, req.Status)
	}

	content, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read content from URL %s: %w", resourceURL, err)
	}

	if len(content) == 0 {
		p.logger.Warn("No content received from URL", "url", resourceURL)
		return nil, ErrNoContent
	}
	p.logger.Debug("Content successfully read from URL", "url", resourceURL, "length", len(content))

	var station Station
	if err := json.Unmarshal(content, &station); err != nil {
		return nil, ErrUnmarshalFailed
	}

	p.logger.Info("Successfully fetched station", "id", id)
	return &station, nil
}

func (p *HTTPProvider) GetStationWaterLevel(ctx context.Context, id string) (*WaterLevelCollection, error) {
	defer ctx.Done()

	if p.baseURL == "" {
		return nil, ErrBaseURLNotSet
	}

	if p.client == nil {
		return nil, ErrHTTPClientNotSet
	}

	if id == "" {
		return nil, ErrInvalidStationID
	}

	resourceURL := fmt.Sprintf("%s/stations/%s/W/measurements.json", p.baseURL, id)
	req, err := p.client.Get(resourceURL)
	if err != nil {
		return nil, err
	}

	defer req.Body.Close()

	if req.StatusCode != http.StatusOK {
		p.logger.Error("Failed to fetch water level for station %s: %s", id, req.Status)

		if req.StatusCode == http.StatusNotFound {
			return nil, ErrResourceNotFound
		}

		return nil, fmt.Errorf("failed to fetch water level for station %s: %s", id, req.Status)
	}

	if req.ContentLength == 0 {
		p.logger.Warn("No content received from URL", "url", resourceURL)
		return nil, ErrNoContent
	}

	p.logger.Debug("Processing content from URL", "url", resourceURL)
	content, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read content from URL %s: %w", resourceURL, err)
	}

	if len(content) == 0 {
		p.logger.Warn("No content received from URL", "url", resourceURL)
		return nil, ErrNoContent
	}
	p.logger.Debug("Content successfully read from URL", "url", resourceURL, "length", len(content))

	measurements := MeasurementList{}
	if err := json.Unmarshal(content, &measurements); err != nil {
		return nil, ErrUnmarshalFailed
	}

	p.logger.Info("Successfully fetched water level for station", "id", id)
	return &WaterLevelCollection{
		StationID:    id,
		Start:        DefaultTimePeriod, // Default start period
		Measurements: measurements,
	}, nil
}
