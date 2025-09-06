package task

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/timgluz/wasserspiegel/measurement"
	"github.com/timgluz/wasserspiegel/station"
)

type StationWaterLevelCollector struct {
	measurementRepo measurement.Repository
	stationRepo     station.Repository
	stationProvider station.Provider

	logger *slog.Logger
}

func NewStationWaterLevelCollector(measurementRepo measurement.Repository,
	stationRepo station.Repository,
	stationProvider station.Provider,
	logger *slog.Logger,
) *StationWaterLevelCollector {
	return &StationWaterLevelCollector{measurementRepo, stationRepo, stationProvider, logger}
}

func (t *StationWaterLevelCollector) Run(ctx context.Context, stationID string, period measurement.Period) error {
	defer ctx.Done()
	t.logger.Info("Fetching water level data for station", "stationID", stationID, "period", period.String())

	stationDetails, err := t.stationRepo.GetByID(ctx, stationID)
	if err != nil {
		t.logger.Error("Failed to fetch station details", "error", err)
		return err
	}

	if stationDetails == nil {
		t.logger.Error("Station not found", "stationID", stationID)
		return fmt.Errorf("station not found: %s", stationID)
	}

	if stationDetails.IsDisabled {
		t.logger.Info("Station is disabled, skipping water level collection", "stationID", stationID)
		return nil
	}

	pegelOnlineID, ok := stationDetails.GetPegelOnlineID()
	if !ok || pegelOnlineID == "" {
		t.logger.Error("Station does not have a valid PegelOnline ID", "stationID", stationID)
		return fmt.Errorf("station does not have a valid PegelOnline ID")
	}

	// Fetch the water level data from the provider
	// TODO: use smaller period or use period from database
	t.logger.Debug("Fetching water levels from provider", "pegelOnlineID", pegelOnlineID, "stationID", stationID)
	waterLevels, err := t.stationProvider.GetStationWaterLevel(ctx, pegelOnlineID)
	if err != nil {
		t.logger.Error("Failed to fetch water levels", "error", err)
		return err
	}
	if waterLevels == nil || len(waterLevels.Measurements) == 0 {
		t.logger.Warn("No water levels found for station", "stationID", stationID)
		return nil
	}
	t.logger.Debug("Fetched water levels", "count", len(waterLevels.Measurements), "stationID", stationID)

	t.logger.Debug("Mapping water level collection to timeseries", "stationID", stationID)
	measurementName := measurement.NewMeasurementName("waterlevel", stationID)
	timeseries, err := mapWaterLevelCollectionToTimeseries(waterLevels, measurementName, period)
	if err != nil {
		t.logger.Error("Failed to map water level collection to timeseries", "error", err)
		return err
	}

	// Add the timeseries to the repository
	t.logger.Debug("Adding timeseries to repository", "measurementName", measurementName)
	if err := t.measurementRepo.AddTimeseries(ctx, timeseries); err != nil {
		t.logger.Error("Failed to add timeseries to repository", "error", err)
		return err
	}

	t.logger.Info("Successfully fetched and stored water level data", "stationID", stationID)
	return nil
}

func mapWaterLevelCollectionToTimeseries(waterLevels *station.WaterLevelCollection, measurementName string, period measurement.Period) (*measurement.Timeseries, error) {
	if waterLevels == nil || len(waterLevels.Measurements) == 0 {
		return nil, fmt.Errorf("no water levels available for station %s", waterLevels.StationID)
	}

	samples := make([]measurement.Sample, 0, len(waterLevels.Measurements))
	for i := range waterLevels.Measurements {
		level := waterLevels.Measurements[i]
		sampleEpoch, err := measurement.ParseRFC3339(level.Timestamp)
		if err != nil {
			return nil, err
		}
		samples = append(samples, measurement.Sample{
			Timestamp: sampleEpoch,
			Value:     level.Value,
		})
	}

	return &measurement.Timeseries{
		Name:    measurementName,
		Samples: samples,
		Start:   period.Start,
		End:     period.End,
		Measurement: &measurement.Measurement{
			Name:        measurementName,
			Description: "Water level measurements for station " + waterLevels.StationID,
			Unit:        "cm",
		},
	}, nil
}
