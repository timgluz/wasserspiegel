package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/julienschmidt/httprouter"
	spinhttp "github.com/spinframework/spin-go-sdk/http"
	spinvars "github.com/spinframework/spin-go-sdk/variables"
	"github.com/timgluz/wasserspiegel/secret"
	"github.com/timgluz/wasserspiegel/station"
)

const (
	JSONContentType = "application/json"
	HTMLContentType = "text/html"
)

var (
	ErrFailedToMarshal = fmt.Errorf("failed to marshal data")
	ErrNotFound        = fmt.Errorf("request resource does not exist")
)

type StationAppConfig struct {
	StoreName string `validate:"required"`
	BaseURL   string `validate:"required"`
	ApiKey    string `validate:"required"` // Optional, if needed for authentication
}

func NewStationAppConfigFromSpinVariables() (*StationAppConfig, error) {
	apiKey, err := spinvars.Get("api_key")
	if err != nil {
		return nil, fmt.Errorf("failed to get api_key from Spin variables: %w", err)
	}

	storeName, err := spinvars.Get("store_name")
	if err != nil {
		return nil, fmt.Errorf("failed to get store_name from Spin variables: %w", err)
	}

	baseURL, err := spinvars.Get("base_url")
	if err != nil {
		return nil, fmt.Errorf("failed to get base_url from Spin variables: %w", err)
	}

	return &StationAppConfig{
		StoreName: storeName,
		BaseURL:   baseURL,
		ApiKey:    apiKey,
	}, nil

}

// stationAppSystem holds the stateful components for the station service.
// it is inspired by Clojure components library: https://github.com/stuartsierra/component
type stationAppComponent struct {
	stationRepository station.Repository
	stationProvider   station.Provider
	secretStore       secret.Store
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

	if s.secretStore == nil {
		s.logger.Error("Secret store is not initialized")
		return false
	}

	s.logger.Info("Station app component is ready")
	return true
}

type StationDashboard struct {
	Station    *station.Station              `json:"station"`
	WaterLevel *station.WaterLevelCollection `json:"water_level"`
}

func init() {
	spinhttp.Handle(func(w http.ResponseWriter, r *http.Request) {
		config, err := NewStationAppConfigFromSpinVariables()
		if err != nil {
			renderFatal(w, fmt.Errorf("failed to load station app configuration: %w", err))
			return
		}

		appComponents, err := initSystemAppComponent(*config)
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
		router.GET("/stations/:id/waterlevel", bearerAuth(newWaterLevelHandler(appComponents), appComponents.secretStore))
		router.GET("/stations/:id", bearerAuth(newStationHandler(appComponents), appComponents.secretStore))
		router.GET("/stations", bearerAuth(newStationsHandler(appComponents), appComponents.secretStore))

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

	secretStore := secret.NewInMemoryStore()
	if err != nil {
		logger.Error("Failed to create secret store", "error", err)
		return nil, fmt.Errorf("failed to create secret store: %w", err)
	}

	secretStore.Set(config.ApiKey, config.ApiKey)

	return &stationAppComponent{stationRepository, stationProvider, secretStore, logger}, nil
}

func main() {}

func newNotFoundHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Warn("Resource not found", "path", r.URL.Path)
		renderError(w, ErrNotFound, http.StatusNotFound)
	}
}

func bearerAuth(h httprouter.Handle, secretStore secret.Store) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || len(authHeader) < 7 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		authType := strings.ToLower(strings.TrimSpace(authHeader[:7]))
		if authType != "bearer" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unsupported authorization type", http.StatusBadRequest)
			return
		}

		token := strings.TrimSpace(authHeader[7:]) // Extract the token part
		if token == "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if secretStore == nil {
			http.Error(w, "Service is not ready", http.StatusInternalServerError)
			return
		}

		if _, err := secretStore.Get(token); err != nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			if err == secret.ErrSecretNotFound {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			http.Error(w, fmt.Sprintf("Invalid token", err), http.StatusInternalServerError)
			return
		}

		h(w, r, ps)
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
		if stationID == "" {
			renderError(w, ErrNotFound, http.StatusNotFound)
		}

		logger.Debug("Fetching station by ID", "id", stationID)
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

		waterLevelCollection, err := fetchCachedWaterLevels(appComponents, stationID)
		if err != nil {
			logger.Error("Failed to fetch water levels for station", "id", stationID, "error", err)
			renderFatal(w, err)
			return
		}

		stationDashboard := &StationDashboard{
			Station:    station,
			WaterLevel: waterLevelCollection,
		}

		renderJSONResponse(w, stationDashboard)
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

	// augment water level with latest measurement and unit
	waterLevelCollection.Unit = station.UnitCM // Default unit for water level measurements
	waterLevelCollection.Latest = waterLevelCollection.GetLatestMeasurement()
	if err := waterLevelCollection.CalculateTrends(waterLevelCollection.Measurements); err != nil {
		logger.Error("Failed to calculate trends for water levels", "id", stationID, "error", err)
	}

	// TODO: cache water level collection in repository

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

func renderJSONResponse(w http.ResponseWriter, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		renderFatal(w, fmt.Errorf("failed to marshal data: %w", err))
		return
	}

	w.Header().Set("Content-Type", JSONContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jsonData)
}
