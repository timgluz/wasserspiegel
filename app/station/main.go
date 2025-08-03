package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	spinhttp "github.com/spinframework/spin-go-sdk/v2/http"
	spinvars "github.com/spinframework/spin-go-sdk/v2/variables"

	"github.com/timgluz/wasserspiegel/middleware"
	"github.com/timgluz/wasserspiegel/response"
	"github.com/timgluz/wasserspiegel/secret"
	"github.com/timgluz/wasserspiegel/station"
)

const (
	JSONContentType = "application/json"
	HTMLContentType = "text/html"

	DefaultSearchLimit   = 10
	MaxSearchLimit       = 100
	MinSearchQueryLength = 3
	MaxSearchQueryLength = 100
)

var (
	ErrFailedToMarshal = fmt.Errorf("failed to marshal data")
	ErrNotFound        = fmt.Errorf("request resource does not exist")
)

type StationAppConfig struct {
	StoreName   string `validate:"required"`
	APIEndpoint string `validate:"required"`
	APIKey      string `validate:"required"` // Optional, if needed for authentication
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

	apiEndpoint, err := spinvars.Get("api_endpoint")
	if err != nil {
		return nil, fmt.Errorf("failed to get base_url from Spin variables: %w", err)
	}

	return &StationAppConfig{
		StoreName:   storeName,
		APIEndpoint: apiEndpoint,
		APIKey:      apiKey,
	}, nil

}

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
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
			response.RenderFatal(w, fmt.Errorf("failed to load station app configuration: %w", err))
			return
		}

		appComponents, err := initSystemAppComponent(*config)
		if err != nil {
			fmt.Printf("Error initializing station service: %v\n", err)
			response.RenderFatal(w, fmt.Errorf("failed to initialize station app"))
			return
		}
		defer appComponents.Close()

		if !appComponents.IsReady() {
			fmt.Println("Station app components are not ready")
			response.RenderFatal(w, fmt.Errorf("station app components are not ready"))
			return
		}

		logger := appComponents.logger
		logger.Info("Station AppComponents successfully initialized", "storeName", config.StoreName)

		router := spinhttp.NewRouter()
		router.POST("/stations/admin/seed", middleware.BearerAuth(newStationSeederHandler(appComponents), appComponents.secretStore))
		router.GET("/stations", middleware.BearerAuth(newStationsHandler(appComponents), appComponents.secretStore))
		router.GET("/stations/:id/waterlevel/", middleware.BearerAuth(newWaterLevelHandler(appComponents), appComponents.secretStore))
		router.GET("/stations/:id", middleware.BearerAuth(newStationHandler(appComponents), appComponents.secretStore))

		router.NotFound = response.NewNotFoundHandler(logger)

		router.ServeHTTP(w, r)
	})
}

func initSystemAppComponent(config StationAppConfig) (*stationAppComponent, error) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil)).With("component", "station")
	logger.Info("Initializing station service")

	// Initialize the Spin KV repository for station data
	stationRepository, err := station.NewSpinKVRepository(config.StoreName, logger)
	if err != nil {
		logger.Error("Failed to create station repository", "error", err)
		return nil, fmt.Errorf("failed to create station repository: %w", err)
	}

	spinHTTPClient := spinhttp.NewClient()
	stationProvider := station.NewPegelOnlineProvider(config.APIEndpoint, spinHTTPClient, logger)

	secretStore := secret.NewInMemoryStore()
	secretStore.Set(config.APIKey, config.APIKey)

	return &stationAppComponent{stationRepository, stationProvider, secretStore, logger}, nil
}

func main() {}

func newStationsHandler(appComponents *stationAppComponent) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		logger := appComponents.logger

		logger.Debug("Fetching all stations")
		queryPagination := response.NewPaginationFromRequest(r)
		stationCollection, err := fetchCachedStations(appComponents, queryPagination)
		if err != nil {
			logger.Error("Failed to fetch stations", "error", err)
			response.RenderFatal(w, err)
			return
		}

		if stationCollection == nil || len(stationCollection.Stations) == 0 {
			logger.Warn("No stations found in the collection")
			response.RenderError(w, ErrNotFound, http.StatusNotFound)
			return
		}

		queryPagination.Total = len(stationCollection.Stations)
		response.RenderJSONResponse(w, map[string]interface{}{
			"stations":   stationCollection.Stations,
			"pagination": queryPagination,
		})
	}
}

func newStationHandler(appComponents *stationAppComponent) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		logger := appComponents.logger
		stationID := params.ByName("id")
		if stationID == "" {
			response.RenderError(w, ErrNotFound, http.StatusNotFound)
		}

		logger.Debug("Fetching station by ID", "id", stationID)
		stationItem, err := fetchCachedStationByID(appComponents, stationID)
		if err != nil {
			logger.Error("Failed to fetch station by ID", "id", stationID, "error", err)
			response.RenderError(w, ErrNotFound, http.StatusNotFound)
			return
		}

		if stationItem == nil {
			logger.Warn("Station not found", "id", stationID)
			response.RenderError(w, ErrNotFound, http.StatusNotFound)
			return
		}

		waterLevelCollection, err := fetchCachedWaterLevels(appComponents, *stationItem)
		if err != nil {
			logger.Error("Failed to fetch water levels for station", "id", stationID, "error", err)
			waterLevelCollection = &station.WaterLevelCollection{
				StationID:    stationItem.ID,
				Start:        station.DefaultTimePeriod, // Default start period
				End:          station.DefaultTimePeriod, // Default end period
				Unit:         station.UnitCM,            // Default unit for water level measurements
				Measurements: []station.Measurement{},
			}
		}

		stationDashboard := &StationDashboard{
			Station:    stationItem,
			WaterLevel: waterLevelCollection,
		}

		response.RenderJSONResponse(w, stationDashboard)
	}
}

func newWaterLevelHandler(appComponents *stationAppComponent) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		logger := appComponents.logger
		stationID := params.ByName("id")
		logger.Debug("Fetching water level for station", "id", stationID)

		if stationID == "" {
			response.RenderError(w, ErrNotFound, http.StatusNotFound)
			return
		}

		stationItem, err := fetchCachedStationByID(appComponents, stationID)
		if err != nil {
			logger.Error("Failed to fetch station by ID", "id", stationID, "error", err)
			response.RenderFatal(w, err)
			return
		}

		waterLevelCollection, err := fetchCachedWaterLevels(appComponents, *stationItem)
		if err != nil {
			logger.Error("Failed to fetch water levels", "id", stationID, "error", err)
			response.RenderFatal(w, err)
			return
		}

		if waterLevelCollection == nil || len(waterLevelCollection.Measurements) == 0 {
			logger.Warn("No water levels found for station", "id", stationID)
			response.RenderError(w, ErrNotFound, http.StatusNotFound)
			return
		}

		jsonData, err := json.Marshal(waterLevelCollection)
		if err != nil {
			logger.Error("Failed to marshal water level data", "id", stationID, "error", err)
			response.RenderFatal(w, ErrFailedToMarshal)
			return
		}

		response.RenderSuccess(w, jsonData)
	}
}

func newStationSeederHandler(appComponents *stationAppComponent) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, _ spinhttp.Params) {
		logger := appComponents.logger

		if !appComponents.IsReady() {
			logger.Error("Station app components are not ready")
			response.RenderFatal(w, fmt.Errorf("station app components are not ready"))
			return
		}

		seeder := station.NewProviderSeeder(logger)
		if seeder == nil {
			logger.Error("Failed to create station seeder")
			response.RenderFatal(w, fmt.Errorf("failed to create station seeder"))
			return
		}

		logger.Info("Seeding stations from provider to repository")
		err := seeder.Seed(context.Background(), appComponents.stationProvider, appComponents.stationRepository)
		if err != nil {
			logger.Error("Failed to seed stations", "error", err)
			response.RenderFatal(w, err)
			return
		}

		logger.Info("Successfully seeded stations")
		response.RenderJSONResponse(w, APIResponse{
			Success: true,
			Message: "Stations seeded successfully",
		})
	}
}

func fetchCachedStations(appComponents *stationAppComponent, pagination response.Pagination) (*station.StationCollection, error) {
	logger := appComponents.logger
	stationRepository := appComponents.stationRepository

	if !stationRepository.IsReady() {
		return nil, fmt.Errorf("station repository is not ready")
	}

	stationCollection, err := stationRepository.List(context.Background(), pagination.Offset, pagination.Limit)
	if err != nil {
		logger.Error("Failed to get stations from repository", "error", err)
		return nil, err
	}

	logger.Debug("Stations found in repository, returning cached data", "count", len(stationCollection.Stations))
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
	if err != nil {
		return nil, fmt.Errorf("failed to get station by ID: %w", err)
	}

	if err == nil && station != nil {
		logger.Debug("Station found in repository, returning cached data", "id", id)
		return station, nil
	}

	logger.Info("Successfully fetched and cached station", "id", id)
	return station, nil
}

func fetchCachedWaterLevels(appComponents *stationAppComponent, stationItem station.Station) (*station.WaterLevelCollection, error) {
	logger := appComponents.logger

	stationID := stationItem.ID
	pegelOnlineID, ok := stationItem.GetExternalID(station.PegelOnlineProviderName)
	if !ok || pegelOnlineID == "" {
		logger.Error("Station does not have a valid PegelOnline ID", "stationID", stationItem.ID)
		return nil, fmt.Errorf("station does not have a valid PegelOnline ID: %s", stationItem.ID)
	}

	logger.Debug("Fetching water levels for station", "id", stationID, "pegelOnlineID", pegelOnlineID)
	waterLevelCollection, err := appComponents.stationProvider.GetStationWaterLevel(context.Background(), pegelOnlineID)
	if err != nil {
		logger.Error("Failed to fetch water levels from provider", "id", stationID, "pegelOnlineID", pegelOnlineID, "error", err)
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

	logger.Debug("Successfully fetched water levels for station", "id", stationID, "count", len(waterLevelCollection.Measurements))
	return waterLevelCollection, nil
}
