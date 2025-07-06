package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	spinhttp "github.com/spinframework/spin-go-sdk/http"
	"github.com/timgluz/wasserspiegel/station"
)

const (
	JSONContentType = "application/json"

	// PegelOnlineBaseURL is the base URL for the PegelOnline API.
	PegelOnlineBaseURL = "https://www.pegelonline.wsv.de/webservices/rest-api/v2"
)

var (
	ErrFailedToMarshal = fmt.Errorf("failed to marshal data")
	ErrNotFound        = fmt.Errorf("request resource does not exist")
)

type StationAppConfig struct {
	StoreName string `validate:"required"`
	BaseURL   string `validate:"required"`
}

// stationAppSystem holds the stateful components for the station service.
// it is inspired by Clojure components library: https://github.com/stuartsierra/component
type stationAppComponent struct {
	stationRepository station.Repository
	stationProvider   station.Provider
	logger            *slog.Logger
}

func (s *stationAppComponent) Close() {
	if s.stationRepository != nil {
		return
	}

	if err := s.stationRepository.Close(); err != nil {
		s.logger.Error("Failed to close station repository", "error", err)
	}
	s.stationRepository = nil
}

// IsReady checks if all components of the station app are ready.
func (s *stationAppComponent) IsReady() bool {
	if s.logger == nil {
		fmt.Println("Logger of stationAppComponent is not initialized")
		return false
	}

	if s.stationRepository == nil {
		s.logger.Error("Station repository is not initialized")
		return false
	}

	if !s.stationRepository.IsReady() {
		s.logger.Error("Station repository is not ready")
		return false
	}

	if s.stationProvider == nil {
		s.logger.Error("Station provider is not initialized")
		return false
	}

	if !s.stationProvider.IsReady() {
		s.logger.Error("Station provider is not ready")
		return false
	}

	s.logger.Info("Station app component is ready")
	return true
}

func init() {
	spinhttp.Handle(func(w http.ResponseWriter, r *http.Request) {
		//TODO: load configuration from environment or other sources
		config := StationAppConfig{
			StoreName: "stations",
			BaseURL:   PegelOnlineBaseURL,
		}

		appComponents, err := initSystemAppComponent(config)
		if err != nil {
			fmt.Printf("Error initializing station service: %v\n", err)
			return
		}
		defer appComponents.Close()

		if !appComponents.IsReady() {
			fmt.Println("Station app components are not ready")
			renderFatal(w, fmt.Errorf("station app components are not ready"))
			return
		}

		logger := appComponents.logger
		logger.Info("Station AppComponents successfully initialized", "storeName", config.StoreName)

		router := spinhttp.NewRouter()
		router.GET("/stations/:id/waterlevel", newWaterLevelHandler(appComponents))
		router.GET("/stations/:id", newStationHandler(appComponents))
		router.GET("/stations", newStationsHandler(appComponents))
		router.NotFound = newNotFoundHandler(logger)

		router.ServeHTTP(w, r)
	})
}

func initSystemAppComponent(config StationAppConfig) (*stationAppComponent, error) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	logger.Info("Initializing station service")

	// Initialize the Spin KV repository for station data
	stationRepository, err := station.NewSpinKVRepository(config.StoreName, logger)
	if err != nil {
		logger.Error("Failed to create station repository", "error", err)
		return nil, fmt.Errorf("failed to create station repository: %w", err)
	}

	spinHTTPClient := spinhttp.NewClient()
	stationProvider := station.NewHTTPProvider(config.BaseURL, spinHTTPClient, logger)

	return &stationAppComponent{stationRepository, stationProvider, logger}, nil
}

func main() {}

func newNotFoundHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Warn("Resource not found", "path", r.URL.Path)
		renderError(w, ErrNotFound, http.StatusNotFound)
	}
}

func newStationsHandler(appComponents *stationAppComponent) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, _ spinhttp.Params) {
		logger := appComponents.logger

		logger.Debug("Fetching all stations")
		stationCollection, err := fetchCachedStations(appComponents)
		if err != nil {
			logger.Error("Failed to fetch stations", "error", err)
			renderFatal(w, err)
			return
		}

		if stationCollection == nil || len(stationCollection.Stations) == 0 {
			logger.Warn("No stations found in the collection")
			renderError(w, ErrNotFound, http.StatusNotFound)
			return
		}

		jsonData, err := json.Marshal(stationCollection)
		if err != nil {
			logger.Error("Failed to marshal station data", "error", err)
			renderFatal(w, ErrFailedToMarshal)
			return
		}

		renderSuccess(w, jsonData)
	}
}

func newStationHandler(appComponents *stationAppComponent) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		logger := appComponents.logger
		stationID := params.ByName("id")
		logger.Debug("Fetching station by ID", "id", stationID)
		if stationID == "" {
			renderError(w, ErrNotFound, http.StatusNotFound)
		}

		station, err := fetchCachedStationByID(appComponents, stationID)
		if err != nil {
			logger.Error("Failed to fetch station by ID", "id", stationID, "error", err)
			renderFatal(w, err)
			return
		}

		if station == nil {
			logger.Warn("Station not found", "id", stationID)
			renderError(w, ErrNotFound, http.StatusNotFound)
			return
		}

		jsonData, err := json.Marshal(station)
		if err != nil {
			logger.Error("Failed to marshal station data", "id", stationID, "error", err)
			renderFatal(w, ErrFailedToMarshal)
			return
		}

		renderSuccess(w, jsonData)
	}
}

func newWaterLevelHandler(appComponents *stationAppComponent) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		logger := appComponents.logger
		stationID := params.ByName("id")
		logger.Debug("Fetching water level for station", "id", stationID)

		if stationID == "" {
			renderError(w, ErrNotFound, http.StatusNotFound)
			return
		}

		waterLevelCollection, err := fetchCachedWaterLevels(appComponents, stationID)
		if err != nil {
			logger.Error("Failed to fetch water levels", "id", stationID, "error", err)
			renderFatal(w, err)
			return
		}

		if waterLevelCollection == nil || len(waterLevelCollection.Measurements) == 0 {
			logger.Warn("No water levels found for station", "id", stationID)
			renderError(w, ErrNotFound, http.StatusNotFound)
			return
		}

		jsonData, err := json.Marshal(waterLevelCollection)
		if err != nil {
			logger.Error("Failed to marshal water level data", "id", stationID, "error", err)
			renderFatal(w, ErrFailedToMarshal)
			return
		}

		renderSuccess(w, jsonData)
	}
}

func fetchCachedStations(appComponents *stationAppComponent) (*station.StationCollection, error) {
	logger := appComponents.logger
	stationRepository := appComponents.stationRepository

	if !stationRepository.IsReady() {
		return nil, fmt.Errorf("station repository is not ready")
	}

	fmt.Println("Checking if stations exist in repository")
	stationCollection, err := stationRepository.List(context.Background(), nil)
	if err == nil && stationCollection != nil {
		logger.Debug("Stations found in repository, returning cached data", "count", len(stationCollection.Stations))
		return stationCollection, nil
	}

	logger.Warn("No cached stations found in repository, fetching from external provider")
	stationCollection, err = appComponents.stationProvider.GetStations(context.Background())
	if err != nil {
		logger.Error("Failed to fetch stations from provider", "error", err)
		return nil, fmt.Errorf("failed to fetch stations from provider: %w", err)
	}

	if stationCollection == nil || len(stationCollection.Stations) == 0 {
		logger.Warn("No stations found in fetched data, check if data schema has changed")
		return nil, fmt.Errorf("no stations found in fetched data")
	}

	logger.Debug("Storing stations to repository", "count", len(stationCollection.Stations))
	err = stationRepository.CreateList(context.Background(), stationCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to add stations to repository: %w", err)
	}

	logger.Info("Successfully fetched and cached stations", "count", len(stationCollection.Stations))
	return stationCollection, nil
}

func fetchCachedStationByID(appComponents *stationAppComponent, id string) (*station.Station, error) {
	logger := appComponents.logger
	stationRepository := appComponents.stationRepository

	if !stationRepository.IsReady() {
		return nil, fmt.Errorf("station repository is not ready")
	}

	if id == "" {
		return nil, fmt.Errorf("station ID cannot be empty")
	}

	logger.Debug("Checking if station exists in repository", "id", id)
	station, err := stationRepository.GetByID(context.Background(), id)
	if err == nil && station != nil {
		logger.Debug("Station found in repository, returning cached data", "id", id)
		return station, nil
	}

	logger.Warn("No cached station found in repository, fetching from external provider")
	station, err = appComponents.stationProvider.GetStation(context.Background(), id)
	if err != nil {
		logger.Error("Failed to fetch station from provider", "id", id, "error", err)
		return nil, fmt.Errorf("failed to fetch station from provider: %w", err)
	}

	if station == nil {
		logger.Warn("Station not found in fetched data", "id", id)
		return nil, fmt.Errorf("station not found with ID: %s", id)
	}

	logger.Debug("Storing station to repository", "id", id)
	err = stationRepository.Create(context.Background(), station)
	if err != nil {
		return nil, fmt.Errorf("failed to add station to repository: %w", err)
	}

	logger.Info("Successfully fetched and cached station", "id", id)
	return station, nil
}

func fetchCachedWaterLevels(appComponents *stationAppComponent, stationID string) (*station.WaterLevelCollection, error) {
	logger := appComponents.logger

	waterLevelCollection, err := appComponents.stationProvider.GetStationWaterLevel(context.Background(), stationID)
	if err != nil {
		logger.Error("Failed to fetch water levels from provider", "id", stationID, "error", err)
		return nil, fmt.Errorf("failed to fetch water levels from provider: %w", err)
	}

	if waterLevelCollection == nil || len(waterLevelCollection.Measurements) == 0 {
		logger.Warn("No water levels found for station", "id", stationID)
		return nil, fmt.Errorf("no water levels found for station with ID: %s", stationID)
	}

	logger.Debug("Successfully fetched water levels for station", "id", stationID, "count", len(waterLevelCollection.Measurements))
	return waterLevelCollection, nil
}

func renderFatal(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", JSONContentType)

	jsonError := fmt.Sprintf(`{"error": "%s"}`, err.Error())
	http.Error(w, jsonError, http.StatusInternalServerError)
}

func renderError(w http.ResponseWriter, err error, statusCode int) {
	w.Header().Set("Content-Type", JSONContentType)

	jsonError := fmt.Sprintf(`{"error": "%s"}`, err.Error())
	http.Error(w, jsonError, statusCode)
}

func renderSuccess(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", JSONContentType)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
