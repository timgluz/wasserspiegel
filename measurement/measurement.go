package measurement

import (
	"strings"

	"github.com/gosimple/slug"
)

type Epoch int64

type Measurement struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"` // Optional field for additional info
	Unit        string `json:"unit"`                  // Unit of measurement (e.g., "Celsius", "Pascal", "liters")
}

func NewMeasurementName(keys ...string) string {
	// Create a measurement name by joining the keys with a hyphen
	// This is useful for creating unique measurement names based on multiple keys
	return slug.Make(strings.Join(keys, "-"))
}

type Sample struct {
	ID            int64   `json:"id"`
	MeasurementID int64   `json:"measurement_id"`
	Value         float64 `json:"value"`
	Timestamp     Epoch   `json:"timestamp"` // ISO 8601 format
}

type Timeseries struct {
	Name    string   `json:"name"`
	Samples []Sample `json:"samples"`
	Start   Epoch    `json:"start"` // epoch time in seconds
	End     Epoch    `json:"end"`   // epoch time in seconds

	Measurement *Measurement `json:"measurement,omitempty"` // Optional field to include measurement details
}
