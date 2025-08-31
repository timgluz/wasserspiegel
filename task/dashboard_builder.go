package task

import (
	"context"
	"log/slog"

	"github.com/timgluz/wasserspiegel/dashboard"
	"github.com/timgluz/wasserspiegel/measurement"
	"github.com/timgluz/wasserspiegel/station"
)

const (
	DefaultPeriod       = "P15D" // ISO 8601 duration for the last 15 days
	DefaultLanguageCode = "en"
	DefaultTimezone     = "utc"
)

type DashboardBuilderOptions struct {
	StationID    string
	Period       string
	LanguageCode string
	Timezone     string
}

func NewDefaultDashboardBuilderOptions(stationID string) DashboardBuilderOptions {
	return DashboardBuilderOptions{
		StationID:    stationID,
		Period:       DefaultPeriod,
		LanguageCode: DefaultLanguageCode,
		Timezone:     DefaultTimezone,
	}
}

type DashboardBuilder struct {
	stationRepo     station.Repository
	dashboardRepo   dashboard.Repository
	measurementRepo measurement.Repository

	logger *slog.Logger
}

func NewDashboardBuilder(
	stationRepo station.Repository,
	dashboardRepo dashboard.Repository,
	measurementRepo measurement.Repository,
	logger *slog.Logger,
) *DashboardBuilder {
	return &DashboardBuilder{
		stationRepo:     stationRepo,
		dashboardRepo:   dashboardRepo,
		measurementRepo: measurementRepo,
		logger:          logger,
	}
}

func (b *DashboardBuilder) Run(ctx context.Context, opts DashboardBuilderOptions) error {
	defer ctx.Done()
	b.logger.Info("Building dashboards...")

	newDashboard := dashboard.NewEmptyDashboard(opts.StationID, opts.LanguageCode, opts.Timezone)

	dashboardID, err := dashboard.GenerateDashboardID(newDashboard)
	if err != nil {
		b.logger.Error("Failed to generate dashboard ID", "error", err)
		return err
	}

	if existingDashboard, err := b.dashboardRepo.GetByID(ctx, dashboardID); err == nil && existingDashboard != nil {
		b.logger.Info("Existing dashboard found, merging data", "stationID", opts.StationID, "languageCode", opts.LanguageCode)
		newDashboard.Merge(existingDashboard)
	} else {
		b.logger.Info("No existing dashboard found, creating a new one", "stationID", opts.StationID, "languageCode", opts.LanguageCode)
		if err := b.addStationDetails(newDashboard, opts.StationID); err != nil {
			b.logger.Error("Failed to add station details to dashboard", "error", err)
			return err
		}

		newDashboard.Name = "Dashboard for " + newDashboard.Station.Name
		newDashboard.Description = "Auto-generated dashboard for station " + newDashboard.Station.Name
	}

	// Fetch water level measurements
	period, err := measurement.NewFromISO8601Duration(opts.Period)
	if err != nil {
		b.logger.Error("Failed to parse period", "error", err)
		return err
	}

	measurementName := measurement.NewMeasurementName("waterlevel", opts.StationID)
	waterLevelTimeseries, err := b.measurementRepo.GetTimeseries(ctx, measurementName, *period)
	if err != nil {
		b.logger.Error("Failed to fetch water level timeseries", "error", err)
		return err
	}

	newDashboard.WaterLevel = *waterLevelTimeseries

	// store the updated dashboard
	// TODO: if pattern repeats, refactor into upsert method in repository
	if !newDashboard.IsSaved() {
		newDashboard.ID = dashboardID
		if err := b.dashboardRepo.Add(ctx, newDashboard); err != nil {
			b.logger.Error("Failed to add new dashboard", "error", err)
			return err
		}
		b.logger.Info("Added new dashboard", "dashboardID", newDashboard.ID)
	} else {
		if err := b.dashboardRepo.Update(ctx, newDashboard); err != nil {
			b.logger.Error("Failed to update existing dashboard", "error", err)
			return err
		}
		b.logger.Info("Updated existing dashboard", "dashboardID", newDashboard.ID)
	}

	b.logger.Info("Dashboard building process completed")
	return nil
}

func (b *DashboardBuilder) addStationDetails(dashboard *dashboard.Dashboard, stationID string) error {
	stationDetails, err := b.stationRepo.GetByID(context.Background(), stationID)
	if err != nil {
		b.logger.Error("Failed to fetch station details", "error", err)
		return err
	}
	if stationDetails == nil {
		b.logger.Error("Station details not found", "stationID", stationID)
		return nil
	}

	dashboard.Station = *stationDetails

	return nil
}
