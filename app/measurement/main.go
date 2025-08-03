package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	spinhttp "github.com/spinframework/spin-go-sdk/v2/http"
	spinvars "github.com/spinframework/spin-go-sdk/v2/variables"

	"github.com/timgluz/wasserspiegel/measurement"
	"github.com/timgluz/wasserspiegel/middleware"
	"github.com/timgluz/wasserspiegel/response"
	"github.com/timgluz/wasserspiegel/secret"
)

type MeasurementAppConfig struct {
	DBName string `json:"db_name"`
	APIKey string `json:"api_key"`
}

func NewMeasurementAppConfigFromSpinVariables() (*MeasurementAppConfig, error) {
	dbName, err := spinvars.Get("measurement_db_name")
	if err != nil {
		return nil, fmt.Errorf("failed to get measurement_db_name: %w", err)
	}

	apiKey, err := spinvars.Get("api_key")
	if err != nil {
		return nil, fmt.Errorf("failed to get api_key: %w", err)
	}

	return &MeasurementAppConfig{
		DBName: dbName,
		APIKey: apiKey,
	}, nil

}

type measurementAppComponent struct {
	config                *MeasurementAppConfig
	measurementRepository measurement.Repository
	secretStore           secret.Store
	logger                *slog.Logger
}

func (c *measurementAppComponent) IsReady() bool {
	if c.logger == nil {
		fmt.Println("Logger of measurementAppComponent is not initialized")
		return false
	}

	if c.config == nil {
		c.logger.Error("MeasurementAppConfig is not initialized")
		return false
	}

	if c.measurementRepository == nil {
		c.logger.Error("Measurement repository is not initialized")
		return false
	}

	if !c.measurementRepository.IsReady() {
		c.logger.Error("Measurement repository is not ready")
		return false
	}

	if c.secretStore == nil {
		c.logger.Error("Secret store is not initialized")
		return false
	}

	return true
}

func (c *measurementAppComponent) Close() {
	if c.measurementRepository != nil {
		if err := c.measurementRepository.Close(); err != nil {
			c.logger.Error("Failed to close measurement repository", "error", err)
		}
	}

	if c.secretStore != nil {
		if err := c.secretStore.Close(); err != nil {
			c.logger.Error("Failed to close secret store", "error", err)
		}
	}

	c.logger.Info("Measurement app component closed")
}

func init() {
	spinhttp.Handle(func(w http.ResponseWriter, r *http.Request) {
		config, err := NewMeasurementAppConfigFromSpinVariables()
		if err != nil {
			response.RenderFatal(w, fmt.Errorf("failed to load measurement app config: %w", err))
			return
		}

		appComponents, err := initMeasurementAppComponent(*config)
		if err != nil {
			response.RenderFatal(w, fmt.Errorf("failed to initialize measurement app component: %w", err))
			return
		}
		defer appComponents.Close()

		// Check if the app components are ready
		if !appComponents.IsReady() {
			response.RenderFatal(w, fmt.Errorf("measurement app component is not ready"))
			return
		}

		logger := appComponents.logger
		logger.Info("Measurement app component is ready")

		router := spinhttp.NewRouter()
		router.POST("/measurements", middleware.BearerAuth(newMeasurementCreationHandler(appComponents), appComponents.secretStore))
		router.GET("/measurements", middleware.BearerAuth(newMeasurementListHandler(appComponents), appComponents.secretStore))
		router.POST("/measurements/:name", middleware.BearerAuth(newTimeseriesCreationHandler(appComponents), appComponents.secretStore))
		router.GET("/measurements/:name", middleware.BearerAuth(newGetTimeseriesHandler(appComponents), appComponents.secretStore))
		router.NotFound = response.NewNotFoundHandler(logger)
	})
}

func main() {}

func initMeasurementAppComponent(config MeasurementAppConfig) (*measurementAppComponent, error) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil)).With("component", "measurement")
	logger.Info("Initializing measurement app component")

	// Initialize the secret store
	secretStore := secret.NewInMemoryStore()
	secretStore.Set(config.APIKey, config.APIKey)

	// Initialize the measurement repository
	db, err := measurement.NewSpinSqliteDB(config.DBName)
	if err != nil {
		logger.Error("Failed to initialize SQLite DB", "error", err)
		return nil, fmt.Errorf("failed to initialize SQLite DB: %w", err)
	}

	measurementRepository, err := measurement.NewSqlRepository(db, logger)
	if err != nil {
		logger.Error("Failed to initialize measurement repository", "error", err)
		return nil, fmt.Errorf("failed to initialize measurement repository: %w", err)
	}

	return &measurementAppComponent{
		config:                &config,
		measurementRepository: measurementRepository,
		secretStore:           secretStore,
		logger:                logger,
	}, nil
}

func newMeasurementCreationHandler(appComponents *measurementAppComponent) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {

		logger := appComponents.logger
		logger.Debug("Saving new measurement")
		measurement, err := newMeasurementFromRequest(r)
		if err != nil {
			response.RenderError(w, fmt.Errorf("failed to create measurement from request: %w", err), http.StatusBadRequest)
			return
		}

		if err := appComponents.measurementRepository.AddMeasurement(r.Context(), measurement); err != nil {
			logger.Error("Failed to add measurement", "error", err)
			response.RenderError(w, fmt.Errorf("failed to add measurement: %w", err), http.StatusInternalServerError)
			return
		}

		logger.Info("Measurement added successfully", "measurement_id", measurement.ID)
		response.RenderJSON(w, response.NewPostResponse(true, "new measurement added successfully", nil))
	}
}

func newMeasurementListHandler(appComponents *measurementAppComponent) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		logger := appComponents.logger
		logger.Debug("Listing all measurements")

		measurements, err := appComponents.measurementRepository.GetMeasurements(r.Context())
		if err != nil {
			logger.Error("Failed to get all measurements", "error", err)
			response.RenderError(w, fmt.Errorf("failed to get all measurements: %w", err), http.StatusInternalServerError)
			return
		}

		if len(measurements) == 0 {
			logger.Info("No measurements found")
			response.RenderSuccess(w, []byte("[]"))
			return
		}

		logger.Info("Measurements retrieved successfully", "count", len(measurements))
		response.RenderJSON(w, response.NewCollectionResponse(measurements, nil))
	}
}

func newTimeseriesCreationHandler(appComponents *measurementAppComponent) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		measurementName := params.ByName("name")
		if measurementName == "" {
			response.RenderError(w, fmt.Errorf("measurement name is required"), http.StatusBadRequest)
			return
		}

		logger := appComponents.logger
		logger.Debug("Saving timeseries for measurement", "name", measurementName)

		timeseries, err := newTimeseriesFromRequest(r, measurementName)
		if err != nil {
			response.RenderError(w, fmt.Errorf("failed to create timeseries from request: %w", err), http.StatusBadRequest)
			return
		}

		if err := appComponents.measurementRepository.AddTimeseries(r.Context(), timeseries); err != nil {
			logger.Error("Failed to add timeseries", "error", err)
			response.RenderError(w, fmt.Errorf("failed to add timeseries: %w", err), http.StatusInternalServerError)
			return
		}

		logger.Info("Timeseries added successfully", "measurement_name", measurementName)
		response.RenderJSON(w, response.NewPostResponse(true, "new timeseries added successfully", nil))
	}
}

func newGetTimeseriesHandler(appComponents *measurementAppComponent) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		measurementName := params.ByName("name")
		if measurementName == "" {
			response.RenderError(w, fmt.Errorf("measurement name is required"), http.StatusBadRequest)
			return
		}

		period, err := getPeriodFromRequest(r)
		if err != nil {
			response.RenderError(w, fmt.Errorf("invalid period: %w", err), http.StatusBadRequest)
			return

		}
		logger := appComponents.logger
		logger.Debug("Getting timeseries for measurement", "name", measurementName)

		timeseries, err := appComponents.measurementRepository.GetTimeseries(r.Context(), measurementName, *period)
		if err != nil {
			logger.Error("Failed to get timeseries", "error", err)
			response.RenderError(w, fmt.Errorf("failed to get timeseries: %w", err), http.StatusInternalServerError)
			return
		}

		if timeseries == nil {
			logger.Info("No timeseries found for measurement", "name", measurementName)
			response.RenderJSON(w, []measurement.Timeseries{})
			return
		}

		logger.Info("Timeseries retrieved successfully", "measurement_name", measurementName)
		response.RenderJSON(w, timeseries)
	}
}

func newMeasurementFromRequest(r *http.Request) (*measurement.Measurement, error) {
	var m measurement.Measurement
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("failed to decode measurement from request: %w", err)
	}
	r.Body.Close()

	return &m, nil
}

func getPeriodFromRequest(r *http.Request) (*measurement.Period, error) {
	periodString := r.URL.Query().Get("period")
	if periodString != "" {
		period, err := measurement.NewFromISO8601Duration(periodString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse period from request: %w", err)
		}
		if !period.IsValid() {
			return nil, fmt.Errorf("invalid period: start must be before end")
		}
		return period, nil
	}

	startString := r.URL.Query().Get("start")
	endString := r.URL.Query().Get("end")
	if startString == "" || endString == "" {
		return nil, fmt.Errorf("start and end parameters are required")
	}

	period := measurement.Period{}
	startEpoch, err := measurement.ParseEpoch(startString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse start epoch: %w", err)
	}
	period.Start = startEpoch
	period.End = measurement.CurrentEpoch()

	if endString != "" {
		endEpoch, err := measurement.ParseEpoch(endString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse end epoch: %w", err)
		}
		period.End = endEpoch
	}

	if !period.IsValid() {
		return nil, fmt.Errorf("invalid period: start must be before end")
	}

	return &period, nil
}

func newTimeseriesFromRequest(r *http.Request, measurementName string) (*measurement.Timeseries, error) {
	var timeseries measurement.Timeseries
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&timeseries); err != nil {
		return nil, fmt.Errorf("failed to decode timeseries from request: %w", err)
	}
	r.Body.Close()

	timeseries.Name = measurementName
	if timeseries.Measurement == nil {
		timeseries.Measurement = &measurement.Measurement{Name: measurementName}
	}

	return &timeseries, nil
}
