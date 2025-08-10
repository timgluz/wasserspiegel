package dashboard

import (
	"github.com/timgluz/wasserspiegel/measurement"
	"github.com/timgluz/wasserspiegel/station"
)

type Dashboard struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Station     station.Station        `json:"station"`
	WaterLevel  measurement.Timeseries `json:"water_level"`

	CreatedAt int `json:"created_at"`
	UpdatedAt int `json:"updated_at"`
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
	if other.CreatedAt > 0 {
		d.CreatedAt = other.CreatedAt
	}
	if other.UpdatedAt > 0 {
		d.UpdatedAt = other.UpdatedAt
	}
}
