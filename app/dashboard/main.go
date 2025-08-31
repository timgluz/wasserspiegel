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
	"github.com/timgluz/wasserspiegel/middleware"
	"github.com/timgluz/wasserspiegel/response"
	"github.com/timgluz/wasserspiegel/secret"
)

type DashboardAppConfig struct {
	StoreName string `json:"storeName"`
	APIKey    string
	LogLevel  string `json:"logLevel"`
}

type DashboardApp struct {
	Config      *DashboardAppConfig
	Logger      *slog.Logger
	SecretStore secret.Store
	Repository  dashboard.Repository
	Router      *spinhttp.Router
}

func newDashboardAppConfigFromSpinVariables() *DashboardAppConfig {
	storeName, err := spinvars.Get("dashboard_store_name")
	if err != nil {
		return nil
	}

	apiKey, err := spinvars.Get("api_key")
	if err != nil {
		return nil
	}

	logLevel, err := spinvars.Get("log_level")
	if err != nil {
		logLevel = "info"
	}

	return &DashboardAppConfig{
		StoreName: storeName,
		APIKey:    apiKey,
		LogLevel:  logLevel,
	}
}

func init() {
	spinhttp.Handle(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Initializing dashboard app...")
		app, err := initDashboardApp()
		if err != nil {
			response.RenderFatal(w, fmt.Errorf("failed to initialize dashboard app: %w", err))
			return
		}

		if app == nil {
			response.RenderFatal(w, fmt.Errorf("failed to initialize dashboard app"))
			return
		}

		app.Router.ServeHTTP(w, r)
	})
}

func initDashboardApp() (*DashboardApp, error) {
	config := newDashboardAppConfigFromSpinVariables()
	if config == nil {
		return nil, fmt.Errorf("failed to load dashboard app configuration")
	}
	logger := newLogger(config)
	secretStore := newSecretStore(config)
	if secretStore == nil {
		return nil, fmt.Errorf("failed to create secret store")
	}
	dashboardRepo, err := newDashboardRepository(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create dashboard repository: %w", err)
	}
	router := newDashboardRouter(dashboardRepo, secretStore, logger)
	if router == nil {
		return nil, fmt.Errorf("failed to create dashboard router")
	}

	return &DashboardApp{
		Config:      config,
		Logger:      logger,
		SecretStore: secretStore,
		Repository:  dashboardRepo,
		Router:      router,
	}, nil

}

func newDashboardRouter(dashboardRepo dashboard.Repository, secretStore secret.Store, logger *slog.Logger) *spinhttp.Router {

	router := spinhttp.NewRouter()
	router.GET("/dashboards/:id", middleware.BearerAuth(newDashboardGetHandler(dashboardRepo, logger), secretStore))
	router.GET("/dashboards", middleware.BearerAuth(newDashboardIndexHandler(dashboardRepo, logger), secretStore))

	router.NotFound = response.NewNotFoundHandler(logger)

	return router
}

func newDashboardIndexHandler(dashboardRepo dashboard.Repository, logger *slog.Logger) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		pagination := response.NewPaginationFromRequest(r)
		logger.Info("Handling dashboard index request", "limit", pagination.Limit, "offset", pagination.Offset)
		dashboards, err := dashboardRepo.List(r.Context(), pagination.Offset, pagination.Limit)
		if err != nil {
			logger.Error("Failed to list dashboards", "error", err)
			response.RenderError(w, fmt.Errorf("failed to list dashboards: %w", err), http.StatusInternalServerError)
			return
		}

		pagination.Total = len(dashboards)
		dashboardCollection := NewDashboardListCollection(dashboards, pagination)
		response.RenderJSON(w, dashboardCollection)
	}
}

func newDashboardGetHandler(dashboardRepo dashboard.Repository, logger *slog.Logger) spinhttp.RouterHandle {
	return func(w http.ResponseWriter, r *http.Request, params spinhttp.Params) {
		dashboardID := params.ByName("id")
		if dashboardID == "" {
			response.RenderError(w, fmt.Errorf("dashboard ID is required"), http.StatusBadRequest)
			return
		}

		if dashboardRepo == nil {
			logger.Error("Dashboard repository is not ready")
			response.RenderError(w, fmt.Errorf("dashboard repository is not ready"), http.StatusInternalServerError)
			return
		}

		dashboard, err := dashboardRepo.GetByID(r.Context(), dashboardID)
		if err != nil {
			logger.Error("Failed to get dashboard by ID", "id", dashboardID, "error", err)
			response.RenderError(w, fmt.Errorf("failed to get dashboard: %w", err), http.StatusInternalServerError)
			return
		}

		if dashboard == nil {
			response.RenderError(w, fmt.Errorf("dashboard not found"), http.StatusNotFound)
			return
		}

		response.RenderJSON(w, dashboard)
	}
}

func newLogger(config *DashboardAppConfig) *slog.Logger {
	fmt.Println("Creating logger")
	level := slog.LevelInfo
	if config != nil {
		level = log.SlogLevelInfoFromString(config.LogLevel)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	return logger
}

func newSecretStore(config *DashboardAppConfig) secret.Store {
	fmt.Println("Creating secret store with API key")
	if config == nil || config.APIKey == "" {
		fmt.Println("Invalid dashboard app configuration: API key is required")
		return nil
	}

	apiKey := config.APIKey
	store := secret.NewInMemoryStore()
	if err := store.Set(apiKey, apiKey); err != nil {
		return nil
	}
	return store
}

func newDashboardRepository(config *DashboardAppConfig, logger *slog.Logger) (dashboard.Repository, error) {
	fmt.Println("Creating dashboard repository with store name:", config.StoreName)

	if config == nil || config.StoreName == "" {
		return nil, fmt.Errorf("invalid dashboard app configuration: store name is required")
	}

	repo, err := dashboard.NewSpinKVRepository(config.StoreName, logger)
	if err != nil {
		logger.Error("Failed to create dashboard repository", "error", err, "storeName", config.StoreName)
		return nil, fmt.Errorf("failed to create dashboard repository: %w", err)
	}

	return repo, nil
}

type DashboardCollection struct {
	Dashboards []*DashboardListIem `json:"dashboards"`
	Pagination response.Pagination `json:"pagination"`
}

type DashboardListIem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	StationID    string `json:"station_id"`
	LanguageCode string `json:"language_code"`
	Timezone     string `json:"timezone"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

func mapDashboardToListItem(d *dashboard.Dashboard) *DashboardListIem {
	if d == nil {
		return nil
	}

	return &DashboardListIem{
		ID:           d.ID,
		Name:         d.Name,
		Description:  d.Description,
		StationID:    d.Station.ID,
		LanguageCode: d.LanguageCode,
		Timezone:     d.Timezone,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}

func NewDashboardListCollection(dashboards []*dashboard.Dashboard, pagination response.Pagination) *DashboardCollection {
	items := make([]*DashboardListIem, 0, len(dashboards))
	for _, d := range dashboards {
		items = append(items, mapDashboardToListItem(d))
	}

	return &DashboardCollection{
		Dashboards: items,
		Pagination: pagination,
	}
}
