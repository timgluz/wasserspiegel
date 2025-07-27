package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	spinhttp "github.com/spinframework/spin-go-sdk/v2/http"
	spinvars "github.com/spinframework/spin-go-sdk/v2/variables"
	"github.com/timgluz/wasserspiegel/middleware"
	"github.com/timgluz/wasserspiegel/response"
	"github.com/timgluz/wasserspiegel/secret"
	"github.com/timgluz/wasserspiegel/station"
)

const (
	DefaultSearchLimit   = 10
	MaxSearchLimit       = 100
	MinSearchQueryLength = 3
	MaxSearchQueryLength = 100
)

type SearchAppConfig struct {
	StationStoreName string `json:"station_store_name"`
	ApiKey           string
	LogLevel         string `json:"log_level"`
}

func NewSearchAppConfigFromSpinVariables() (*SearchAppConfig, error) {
	stationStoreName, err := spinvars.Get("stations_store_name")
	if err != nil {
		return nil, fmt.Errorf("failed to get stations_store_name: %w", err)
	}

	apiKey, err := spinvars.Get("api_key")
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return &SearchAppConfig{
		StationStoreName: stationStoreName,
		ApiKey:           apiKey,
	}, nil
}

type SearchResponse struct {
	Results []station.Station `json:"results"`
	Total   int               `json:"total"`
	Limit   int               `json:"limit"`
	Offset  int               `json:"offset"`
}

func init() {
	spinhttp.Handle(func(w http.ResponseWriter, r *http.Request) {
		config, err := NewSearchAppConfigFromSpinVariables()
		if err != nil {
			response.RenderFatal(w, fmt.Errorf("failed to load station app configuration: %w", err))
			return
		}

		appComponents, err := initSearchAppComponent(*config)
		if err != nil {
			fmt.Printf("Error initializing station service: %v\n", err)
			return
		}
		defer appComponents.Close()

		if !appComponents.IsReady() {
			fmt.Println("Station app components are not ready")
			response.RenderFatal(w, fmt.Errorf("station app components are not ready"))
			return
		}

		logger := appComponents.logger
		logger.Info("Station AppComponents successfully initialized", "stationStore", config.StationStoreName)

		router := spinhttp.NewRouter()
		router.GET("/search/stations", middleware.BearerAuth(newStationSearchHandler(appComponents), appComponents.secretStore))

		router.NotFound = response.NewNotFoundHandler(logger)

		router.ServeHTTP(w, r)
	})
}

func initSearchAppComponent(config SearchAppConfig) (*searchAppComponent, error) {
	loggerOptions := slog.HandlerOptions{
		Level: slogLevelInfoFromString(config.LogLevel),
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &loggerOptions)).With("component", "search")

	logger.Info("Initializing station service")

	// Initialize the Spin KV repository for station data
	stationRepository, err := station.NewSpinKVRepository(config.StationStoreName, logger)
	if err != nil {
		logger.Error("Failed to create station repository", "error", err, "storeName", config.StationStoreName)
		return nil, fmt.Errorf("failed to create station repository: %w", err)
	}

	secretStore := secret.NewInMemoryStore()
	if err != nil {
		logger.Error("Failed to create secret store", "error", err)
		return nil, fmt.Errorf("failed to create secret store: %w", err)
	}

	secretStore.Set(config.ApiKey, config.ApiKey)

	return &searchAppComponent{
		logger:            logger,
		stationRepository: stationRepository,
		secretStore:       secretStore,
	}, nil
}

func newStationSearchHandler(appComponents *searchAppComponent) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, _ spinhttp.Params) {
		logger := appComponents.logger

		if !appComponents.IsReady() {
			logger.Error("Station app components are not ready")
			response.RenderFatal(w, fmt.Errorf("station app components are not ready"))
			return
		}

		collection, err := appComponents.stationRepository.List(context.Background(), nil)
		if err != nil {
			logger.Error("Failed to fetch stations", "error", err)
			response.RenderFatal(w, err)
			return
		}

		if collection == nil || len(collection.Stations) == 0 {
			logger.Warn("No stations found in the collection")
			response.RenderError(w, response.ErrNotFound, http.StatusNotFound)
			return
		}

		searchQuery := r.URL.Query().Get("q")
		if searchQuery == "" {
			logger.Warn("Search query is empty")
			response.RenderError(w, fmt.Errorf("search query cannot be empty"), http.StatusBadRequest)
			return
		}
		if len(searchQuery) < MinSearchQueryLength {
			logger.Warn("Search query is too short", "query", searchQuery)
			response.RenderError(w, fmt.Errorf("search query must be at least 3 characters long"), http.StatusBadRequest)
			return
		}
		if len(searchQuery) > MaxSearchQueryLength {
			logger.Warn("Search query is too long", "query", searchQuery)
			response.RenderError(w, fmt.Errorf("search query must not exceed 100 characters"), http.StatusBadRequest)
			return
		}

		limitStr := r.URL.Query().Get("limit")
		if limitStr == "" {
			limitStr = "10" // Default limit
		}

		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			logger.Warn("Invalid limit value", "limit", limitStr)
			limit = DefaultSearchLimit // Use a default limit if parsing fails
		}

		if limit > MaxSearchLimit {
			logger.Warn("Limit exceeds maximum allowed value", "limit", limit)
			limit = MaxSearchLimit // Cap the limit to the maximum allowed value
		}

		searchQuery = strings.ToLower(strings.TrimSpace(searchQuery))
		logger.Debug("Searching stations", "query", searchQuery)
		var results []station.Station
		for _, s := range collection.Stations {
			if strings.Contains(strings.ToLower(s.Name), searchQuery) ||
				strings.Contains(strings.ToLower(s.ID), searchQuery) {
				results = append(results, s)
			}
		}

		response.RenderJSONResponse(w, SearchResponse{
			Results: results,
			Total:   len(collection.Stations),
			Limit:   limit,
			Offset:  0, // Offset is not used in this example, but can be implemented if needed
		})
	}
}

type searchAppComponent struct {
	logger            *slog.Logger
	stationRepository station.Repository
	secretStore       secret.Store
}

func (c *searchAppComponent) IsReady() bool {
	if c.logger == nil {
		fmt.Println("Logger of SearchAppComponent is not initialized")
		return false
	}

	if c.stationRepository == nil {
		c.logger.Error("Station repository is not initialized")
		return false
	}

	if c.secretStore == nil {
		c.logger.Error("Secret store is not initialized")
		return false
	}

	c.logger.Debug("SearchAppComponent is ready")
	return true
}

func (c *searchAppComponent) Close() error {
	if c.stationRepository != nil {
		if err := c.stationRepository.Close(); err != nil {
			c.logger.Error("Failed to close station repository", "error", err)
			return err
		}
	}

	c.logger.Info("SearchAppComponent closed successfully")
	return nil
}

func slogLevelInfoFromString(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo // Default to Info level if not recognized
	}
}
