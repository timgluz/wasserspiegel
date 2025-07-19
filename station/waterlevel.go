package station

const (
	DefaultTimePeriod = "P10D" // Default time period for water level data (10 days)
)

const UnitCM = "cm" // Default unit for water level measurements

type MeasurementList []Measurement

type WaterLevelCollection struct {
	StationID    string          `json:"station_id"`
	Start        string          `json:"start"` // Start date or perion in ISO 8601 format, default is P10D (10 days)
	End          string          `json:"end"`   // End date or period in ISO 8601 format, default is P1D (1 day)
	Measurements MeasurementList `json:"measurements"`
	Latest       Measurement     `json:"latest"` // Latest measurement
	Unit         string          `json:"unit"`   // Unit of measurement, e.g., "m" for meters
}

func (wlc *WaterLevelCollection) GetLatestMeasurement() Measurement {
	// Return the latest measurement from the collection
	if len(wlc.Measurements) == 0 {
		return Measurement{}
	}

	// assuming measurements are sorted by timestamp, return the last one
	return wlc.Measurements[len(wlc.Measurements)-1]
}

type Measurement struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

type MeasurementChange struct {
	Period string  `json:"period"` // Period of the change, e.g., "P1D" for 1 day
	Value  float64 `json:"value"`  // Change in value over the period
	Unit   string  `json:"unit"`   // Unit of measurement, e.g., "cm" for centimeters
}

func calculateDifference(m1, m2 Measurement, unit string) MeasurementChange {
	// Calculate the difference between two measurements
	return MeasurementChange{
		Period: "P1D", // Assuming daily change for simplicity
		Value:  m2.Value - m1.Value,
		Unit:   unit,
	}
}
