package station

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

const PegelOnlineProviderName = "pegelonline"

type PegelOnlineStationList []PegelOnlineStation

type PegelOnlineStation struct {
	UUID      string       `json:"uuid"`
	LongName  string       `json:"longname"`
	ShortName string       `json:"shortname"`
	KM        float64      `json:"km"`
	Latitude  float64      `json:"latitude"`
	Longitude float64      `json:"longitude"`
	Water     StationWater `json:"water"`
}

type PegelOnlineProvider struct {
	HTTPProvider

	APIEndpoint string `json:"api_endpoint"`

	logger *slog.Logger
}

type StationWater struct {
	LongName  string `json:"longname"`
	ShortName string `json:"shortname"`
}

func NewPegelOnlineProvider(apiEndpoint string, client *http.Client, logger *slog.Logger) *PegelOnlineProvider {
	return &PegelOnlineProvider{
		APIEndpoint: apiEndpoint,
		logger:      logger,
		HTTPProvider: HTTPProvider{
			client: client,
			logger: logger,
		},
	}
}

func (p *PegelOnlineProvider) IsReady() bool {
	if !p.HTTPProvider.IsReady() {
		return false
	}

	p.logger.Info("PegelOnlineProvider is ready", "APIEndpoint", p.APIEndpoint)
	return true
}

func (p *PegelOnlineProvider) GetStations(ctx context.Context) (*StationCollection, error) {
	defer ctx.Done()

	if !p.IsReady() {
		return nil, ErrProviderNotReady
	}

	resourceURL := p.APIEndpoint + "/stations.json"
	jsonContent, err := p.RetrieveContent(ctx, resourceURL)
	if err != nil {
		return nil, err
	}

	var stations PegelOnlineStationList
	if err := json.NewDecoder(jsonContent).Decode(&stations); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	stationCollection, err := mapStations(stations)
	if err != nil {
		return nil, fmt.Errorf("failed to map stations: %w", err)
	}

	return stationCollection, nil
}

func (p *PegelOnlineProvider) GetStation(ctx context.Context, id string) (*Station, error) {
	defer ctx.Done()

	if !p.IsReady() {
		return nil, ErrProviderNotReady
	}

	resourceURL := fmt.Sprintf("%s/stations/%s.json", p.APIEndpoint, id)
	jsonContent, err := p.RetrieveContent(ctx, resourceURL)
	if err != nil {
		return nil, err
	}

	var station PegelOnlineStation
	if err := json.NewDecoder(jsonContent).Decode(&station); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	mappedStation, err := mapStation(station)
	if err != nil {
		return nil, fmt.Errorf("failed to map station: %w", err)
	}

	p.logger.Info("Successfully fetched station", "id", id, "name", mappedStation.Name)
	return mappedStation, nil
}

// GetStationWaterLevel retrieves the water level for a specific station by its ID.
func (p *PegelOnlineProvider) GetStationWaterLevel(ctx context.Context, id string) (*WaterLevelCollection, error) {
	defer ctx.Done()

	if !p.IsReady() {
		return nil, ErrProviderNotReady
	}

	period := DefaultTimePeriod // Default period is 15 days
	resourceURL := fmt.Sprintf("%s/stations/%s/W/measurements.json?start=%s", p.APIEndpoint, id, period)
	jsonContent, err := p.RetrieveContent(ctx, resourceURL)
	if err != nil {
		return nil, err
	}

	measurements := MeasurementList{}
	if err := json.NewDecoder(jsonContent).Decode(&measurements); err != nil {
		return nil, ErrUnmarshalFailed
	}

	p.logger.Info("Successfully fetched water level for station", "id", id)
	return &WaterLevelCollection{
		StationID:    id,
		Start:        DefaultTimePeriod, // Default start period
		Measurements: measurements,
	}, nil

}

func mapStations(stations PegelOnlineStationList) (*StationCollection, error) {
	stationCollection := &StationCollection{
		Stations: make([]Station, len(stations)),
	}

	for i, station := range stations {
		mappedStation, err := mapStation(station)
		if err != nil {
			return nil, fmt.Errorf("failed to map station: %w", err)
		}
		stationCollection.Stations[i] = *mappedStation
	}

	return stationCollection, nil
}

func mapStation(station PegelOnlineStation) (*Station, error) {
	if station.UUID == "" {
		return nil, ErrInvalidStationID
	}

	if station.LongName == "" || station.Water.LongName == "" {
		return nil, fmt.Errorf("station name or water long name is empty for UUID: %s", station.UUID)
	}

	return &Station{
		ID:    NewStationID(station.Water.LongName, station.LongName),
		Name:  toCapitalize(station.LongName),
		Water: toCapitalize(station.Water.LongName),

		Location: Location{
			KM:        station.KM,
			Latitude:  station.Latitude,
			Longitude: station.Longitude,
		},
		ExternalIDs: []ExternalID{
			{Name: PegelOnlineProviderName, ID: station.UUID},
		},
	}, nil
}

func toCapitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
