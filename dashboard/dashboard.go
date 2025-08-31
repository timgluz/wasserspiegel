package dashboard

import (
	"fmt"
	"strings"

	"github.com/gosimple/slug"
	"github.com/timgluz/wasserspiegel/measurement"
	"github.com/timgluz/wasserspiegel/station"
)

type Dashboard struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Station     station.Station        `json:"station"`
	WaterLevel  measurement.Timeseries `json:"water_level"`

	LanguageCode string `json:"language_code"`
	Timezone     string `json:"timezone"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

func NewEmptyDashboard(stationID, languageCode, timezone string) *Dashboard {
	return &Dashboard{
		ID:           "",
		Name:         "",
		Description:  "",
		Station:      station.Station{ID: stationID},
		WaterLevel:   measurement.Timeseries{},
		LanguageCode: languageCode,
		Timezone:     timezone,
		CreatedAt:    0,
		UpdatedAt:    0,
	}
}

func (d *Dashboard) Merge(other *Dashboard) {
	if other == nil {
		return
	}
	if other.Name != "" {
		d.Name = other.Name
	}
	if other.Description != "" {
		d.Description = other.Description
	}
	if other.Station.ID != "" {
		d.Station = other.Station
	}
	if len(other.WaterLevel.Samples) > 0 {
		d.WaterLevel = other.WaterLevel
	}

	if other.LanguageCode != "" {
		d.LanguageCode = other.LanguageCode
	}

	if other.Timezone != "" {
		d.Timezone = other.Timezone
	}

	if other.CreatedAt > 0 {
		d.CreatedAt = other.CreatedAt
	}
	if other.UpdatedAt > 0 {
		d.UpdatedAt = other.UpdatedAt
	}
}

func (d *Dashboard) IsSaved() bool {
	return d.ID != ""
}

func GenerateDashboardID(dashboard *Dashboard) (string, error) {
	if dashboard == nil {
		return "", fmt.Errorf("dashboard cannot be nil")
	}

	if dashboard.Station.ID == "" {
		return "", fmt.Errorf("dashboard station ID cannot be empty")
	}

	id := strings.Join([]string{dashboard.Station.ID, dashboard.LanguageCode, dashboard.Timezone}, "-")
	return slug.Make(id), nil
}
