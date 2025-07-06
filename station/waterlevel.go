package station

const (
	DefaultTimePeriod = "P10D" // Default time period for water level data (10 days)
)

type MeasurementList []Measurement

type WaterLevelCollection struct {
	StationID    string          `json:"station_id"`
	Start        string          `json:"start"` // Start date or perion in ISO 8601 format, default is P10D (10 days)
	End          string          `json:"end"`   // End date or period in ISO 8601 format, default is P1D (1 day)
	Measurements MeasurementList `json:"measurements"`
}

type Measurement struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}
