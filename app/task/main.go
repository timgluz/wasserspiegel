package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	spinhttp "github.com/spinframework/spin-go-sdk/v2/http"
	spinvars "github.com/spinframework/spin-go-sdk/v2/variables"

	"github.com/timgluz/wasserspiegel/dashboard"
	"github.com/timgluz/wasserspiegel/log"
	"github.com/timgluz/wasserspiegel/measurement"
	"github.com/timgluz/wasserspiegel/middleware"
	"github.com/timgluz/wasserspiegel/response"
	"github.com/timgluz/wasserspiegel/secret"
	"github.com/timgluz/wasserspiegel/station"
	"github.com/timgluz/wasserspiegel/task"
)

type taskAppConfig struct {
	MeasurementDBName  string `json:"measurement_db_name"`
	StationStoreName   string `json:"station_store_name"` // e.g., "stations_store"
	DashboardStoreName string `json:"dashboard_store_name"`

	APIEndpoint       string `json:"api_endpoint"` // e.g., "https://api.pegelonline.wsv.de"
	APIKey            string `json:"api_key"`
	ConnectionTimeout int    `json:"connection_timeout"` // in seconds

	LogLevel string `json:"log_level"`
}

type taskApp struct {
	config taskAppConfig

	measurementRepository measurement.Repository
	dashboardRepository   dashboard.Repository
	stationRepository     station.Repository

	stationProvider station.Provider
	secretStore     secret.Store
	router          *spinhttp.Router

	logger *slog.Logger
}

func init() {
	spinhttp.Handle(func(w http.ResponseWriter, r *http.Request) {
		config, err := newTaskAppConfigFromSpinVariables()
		if err != nil {
			response.RenderFatal(w, fmt.Errorf("failed to load task app configuration: %w", err))
			return
		}

		app, err := initTaskApp(*config)
		if err != nil {
			fmt.Println("Error initializing task app components:", err)
			response.RenderFatal(w, fmt.Errorf("failed to initialize task app components"))
			return
		}
		defer app.Close()

		if !app.IsReady() {
			fmt.Println("Task app components are not ready")
			response.RenderFatal(w, fmt.Errorf("task app components are not ready"))
			return
		}

		router := newTaskRouter(app)

		router.ServeHTTP(w, r)
	})
}

func main() {}

func newTaskRouter(app *taskApp) *spinhttp.Router {
	router := spinhttp.NewRouter()
	router.GET("/tasks/collectStationMeasurements", newCollectStationMeasurementsInfoHandler())
	router.POST("/tasks/collectStationMeasurements", middleware.BearerAuth(newCollectStationMeasurementsHandler(app), app.secretStore))
	router.GET("/tasks/buildDashboard", newBuildDashboardInfoHandler())
	router.POST("/tasks/buildDashboard", middleware.BearerAuth(newBuildDashboardHandler(app), app.secretStore))

	router.NotFound = response.NewNotFoundHandler(app.logger)
	return router
}

// TODO: check if it would be possible to use OPENAPI spec to generate this documentation
func newCollectStationMeasurementsInfoHandler() spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		helpMessage := `Use POST method to collect water level measurements for a station.
Required query parameters:
- station_id (string): The ID of the station to collect measurements for.
Optional query parameters:
- period (ISO 8601 duration, e.g., P3D for 3 days): The time period for which to collect measurements. Default is P3D (3 days) if not provided.`

		response.RenderJSON(w,
			response.NewAPIDocumentationResponse("Collect Station Measurements Info", helpMessage),
		)
	}
}

func newCollectStationMeasurementsHandler(app *taskApp) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		ctx := r.Context()
		logger := app.logger

		// Example: Fetching a specific station ID from the request
		stationID := r.URL.Query().Get("station_id")
		if stationID == "" {
			response.RenderError(w, fmt.Errorf("station id is required"), http.StatusBadRequest)
			return
		}

		periodStr := r.URL.Query().Get("period")
		if periodStr == "" {
			periodStr = "P3D" // Default to 3 days if not provided
		}

		timePeriod, err := measurement.NewFromISO8601Duration(periodStr)
		if err != nil {
			logger.Error("Invalid time period format", "period", periodStr, "error", err)
			response.RenderError(w, fmt.Errorf("invalid time period"), http.StatusBadRequest)
			return
		}

		logger.Info("Collecting water level measurements", "stationID", stationID, "period", timePeriod.String())
		job := task.NewStationWaterLevelCollector(app.measurementRepository,
			app.stationRepository,
			app.stationProvider,
			logger,
		)
		if err := job.Run(ctx, stationID, *timePeriod); err != nil {
			logger.Error("Failed to collect water level measurements", "error", err)
			response.RenderError(w, fmt.Errorf("failed to collect water level measurements: %w", err), http.StatusInternalServerError)
			return
		}

		response.RenderJSON(w, response.NewPostResponse(true, "Water level successfully collected for station: "+stationID, nil))
	}
}

func newBuildDashboardInfoHandler() spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		helpMessage := `Use POST method to build a dashboard for a station.
Required query parameters:
- station_id (string): The ID of the station to build the dashboard for.
Optional query parameters:
- language_code (string): The language code for the dashboard (e.g., "en",
  "de"). Default is "en" if not provided.
- timezone (string): The timezone for the dashboard (e.g., "utc", "Europe/Berlin").
  Default is "utc" if not provided.`

		response.RenderJSON(w,
			response.NewAPIDocumentationResponse("Build Dashboard Info", helpMessage),
		)
	}
}

func newBuildDashboardHandler(app *taskApp) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		ctx := r.Context()
		logger := app.logger

		stationID := r.URL.Query().Get("station_id")
		if stationID == "" {
			response.RenderError(w, fmt.Errorf("station_id is required"), http.StatusBadRequest)
			return
		}

		builderOptions := task.NewDefaultDashboardBuilderOptions(stationID)

		if languageCode := r.URL.Query().Get("language_code"); languageCode != "" {
			builderOptions.LanguageCode = languageCode
		}

		if timezone := r.URL.Query().Get("timezone"); timezone != "" {
			builderOptions.Timezone = timezone
		}

		logger.Info("Building dashboard", "stationID", stationID, "languageCode", builderOptions.LanguageCode, "timezone", builderOptions.Timezone)
		job := task.NewDashboardBuilder(app.stationRepository,
			app.dashboardRepository,
			app.measurementRepository,
			logger,
		)
		if err := job.Run(ctx, builderOptions); err != nil {
			logger.Error("Failed to build dashboard", "error", err)
			response.RenderError(w, fmt.Errorf("failed to build dashboard: %w", err), http.StatusInternalServerError)
			return
		}

		response.RenderJSON(w, response.NewPostResponse(true, "Dashboard successfully built: "+stationID, builderOptions))
	}
}

func newTaskAppConfigFromSpinVariables() (*taskAppConfig, error) {
	measurementDBName, err := spinvars.Get("measurement_db_name")
	if err != nil {
		return nil, fmt.Errorf("failed to get measurement_db_name: %w", err)
	}

	dashboardStoreName, err := spinvars.Get("dashboard_store_name")
	if err != nil {
		return nil, fmt.Errorf("failed to get dashboard_store_name: %w", err)
	}

	stationStoreName, err := spinvars.Get("station_store_name")
	if err != nil {
		return nil, fmt.Errorf("failed to get station_store_name: %w", err)
	}

	apiEndpoint, err := spinvars.Get("api_endpoint")
	if err != nil {
		return nil, fmt.Errorf("failed to get API endpoint: %w", err)
	}

	apiKey, err := spinvars.Get("api_key")
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	logLevel, err := spinvars.Get("log_level")
	if err != nil {
		return nil, fmt.Errorf("failed to get log_level: %w", err)
	}

	return &taskAppConfig{
		MeasurementDBName:  measurementDBName,
		DashboardStoreName: dashboardStoreName,
		StationStoreName:   stationStoreName,
		APIEndpoint:        apiEndpoint,
		APIKey:             apiKey,
		ConnectionTimeout:  10, // Default to 10 seconds if not set
		LogLevel:           logLevel,
	}, nil
}

func initTaskApp(config taskAppConfig) (*taskApp, error) {
	loggerOptions := &slog.HandlerOptions{
		Level: log.SlogLevelInfoFromString(config.LogLevel),
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, loggerOptions)).With("component", "task")
	logger.Info("Initializing Task components")

	measurementDB, err := measurement.NewSpinSqliteDB(config.MeasurementDBName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize SQLite DB: %w", err)
	}

	measurementRepository, err := measurement.NewSqlRepository(measurementDB, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create measurement repository: %w", err)
	}

	stationRepo, err := station.NewSpinKVRepository(config.StationStoreName, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create station repository: %w", err)
	}

	dashboardRepo, err := dashboard.NewSpinKVRepository(config.DashboardStoreName, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create dashboard repository: %w", err)
	}

	httpClient := spinhttp.NewClient()
	stationProvider := station.NewPegelOnlineProvider(config.APIEndpoint, httpClient, logger)

	secretStore := secret.NewInMemoryStore()
	if err != nil {
		return nil, fmt.Errorf("failed to create secret store: %w", err)
	}
	secretStore.Set(config.APIKey, config.APIKey)

	return &taskApp{
		config:                config,
		measurementRepository: measurementRepository,
		dashboardRepository:   dashboardRepo,
		stationRepository:     stationRepo,
		stationProvider:       stationProvider,
		secretStore:           secretStore,
		logger:                logger,
	}, nil
}

func (c *taskApp) IsReady() bool {
	if c.logger == nil {
		fmt.Println("Logger of task app components is not initialized")
		return false
	}

	if c.measurementRepository == nil || !c.measurementRepository.IsReady() {
		c.logger.Error("Measurement repository is not initialized or not ready")
		return false
	}

	if c.dashboardRepository == nil || !c.dashboardRepository.IsReady() {
		c.logger.Error("Dashboard repository is not initialized or not ready")
		return false
	}

	if c.stationRepository == nil || !c.stationRepository.IsReady() {
		c.logger.Error("Station repository is not initialized or not ready")
		return false
	}

	if c.stationProvider == nil || !c.stationProvider.IsReady() {
		c.logger.Error("Station provider is not initialized or not ready")
		return false
	}

	if c.secretStore == nil || !c.secretStore.IsReady() {
		c.logger.Error("Secret store is not initialized or not ready")
		return false
	}

	return true
}

func (c *taskApp) Close() error {
	if c.measurementRepository != nil {
		if err := c.measurementRepository.Close(); err != nil {
			c.logger.Error("Failed to close measurement repository", "error", err)
		}
	}

	if c.dashboardRepository != nil {
		if err := c.dashboardRepository.Close(); err != nil {
			c.logger.Error("Failed to close dashboard repository", "error", err)
		}
	}

	if c.stationRepository != nil {
		if err := c.stationRepository.Close(); err != nil {
			c.logger.Error("Failed to close station repository", "error", err)
		}
	}

	if c.stationProvider != nil {
		if err := c.stationProvider.Close(); err != nil {
			c.logger.Error("Failed to close station provider", "error", err)
		}
	}

	if c.secretStore != nil {
		if err := c.secretStore.Close(); err != nil {
			c.logger.Error("Failed to close secret store", "error", err)
		}
	}

	c.logger.Info("Task app components closed successfully")
	return nil
}
