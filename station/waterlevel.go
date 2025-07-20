package station

import (
	"fmt"
	"time"
)

const (
	DefaultTimePeriod = "P10D" // Default time period for water level data (10 days)
	UnitCM            = "cm"   // Default unit for water level measurements
)

var ErrUnitsMismatch = fmt.Errorf("units mismatch")

type MeasurementList []Measurement

type WaterLevelCollection struct {
	StationID    string           `json:"station_id"`
	Start        string           `json:"start"` // Start date or perion in ISO 8601 format, default is P10D (10 days)
	End          string           `json:"end"`   // End date or period in ISO 8601 format, default is P1D (1 day)
	Measurements MeasurementList  `json:"measurements"`
	Latest       Measurement      `json:"latest"` // Latest measurement
	Trend        MeasurementTrend `json:"trend"`  // changes in over n days
	Unit         string           `json:"unit"`   // Unit of measurement, e.g., "m" for meters
}

func (wlc *WaterLevelCollection) GetLatestMeasurement() Measurement {
	// Return the latest measurement from the collection
	if len(wlc.Measurements) == 0 {
		return Measurement{}
	}

	// assuming measurements are sorted by timestamp, return the last one
	return wlc.Measurements[len(wlc.Measurements)-1]
}

type MeasurementTrend struct {
	P1D *Measurement `json:"p1d,omitempty"` // Difference from the average of measurements from 1 day ago
	P3D *Measurement `json:"p3d,omitempty"` // Difference from the average of measurements from 3 days ago
	P7D *Measurement `json:"p7d,omitempty"` // Difference from the average of measurements from 7 days ago
}

func (wlc *WaterLevelCollection) CalculateTrends(measurements MeasurementList) error {
	if len(measurements) == 0 {
		return fmt.Errorf("no measurements available to calculate trends")
	}

	// Calculate the latest measurement
	wlc.Latest = wlc.GetLatestMeasurement()

	// Calculate trends for P1D, P3D, and P7D
	var err error
	wlc.Trend.P1D, err = calculateDifferenceNDays(wlc.Latest, measurements, 1)
	if err != nil {
		return fmt.Errorf("error calculating P1D trend: %w", err)
	}

	wlc.Trend.P3D, err = calculateDifferenceNDays(wlc.Latest, measurements, 3)
	if err != nil {
		return fmt.Errorf("error calculating P3D trend: %w", err)
	}

	wlc.Trend.P7D, err = calculateDifferenceNDays(wlc.Latest, measurements, 7)
	if err != nil {
		return fmt.Errorf("error calculating P7D trend: %w", err)
	}

	return nil
}

type Measurement struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"` // Unit of measurement, e.g., "cm" for centimeters
}

func (m Measurement) Difference(other Measurement) (*Measurement, error) {
	if m.Unit != other.Unit {
		return nil, ErrUnitsMismatch
	}

	// Calculate the difference between two measurements
	return &Measurement{
		Timestamp: other.Timestamp,
		Value:     other.Value - m.Value,
		Unit:      m.Unit,
	}, nil
}

// calculateDifferenceNDays calculates the difference between a measurement and the average of measurements from n days ago.
// It returns a new Measurement with the calculated difference.
func calculateDifferenceNDays(m Measurement, ms MeasurementList, nDays int) (*Measurement, error) {
	if len(ms) == 0 {
		return nil, fmt.Errorf("no measurements available for P1D difference")
	}

	startDate, err := ParseTimestamp(m.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp format: %w", err)
	}

	targetDate := startDate.AddDate(0, 0, -1*nDays)
	targetMeasurements := getSameDayMeasurements(ms, targetDate)
	if len(targetMeasurements) == 0 {
		return nil, fmt.Errorf("no measurements found for the target date: %s", targetDate.Format(time.RFC3339))
	}

	totalValue := 0.0
	for _, measurement := range targetMeasurements {
		totalValue += measurement.Value
	}
	averageValue := totalValue / float64(len(targetMeasurements))
	return m.Difference(Measurement{
		Timestamp: targetDate.Format(time.RFC3339),
		Value:     averageValue,
		Unit:      m.Unit,
	})
}

func getSameDayMeasurements(ms MeasurementList, targetDate time.Time) MeasurementList {
	var sameDayMeasurements MeasurementList

	for _, m := range ms {
		timestamp, err := ParseTimestamp(m.Timestamp)
		if err != nil {
			continue // Skip invalid timestamps
		}

		if IsDameDay(timestamp, targetDate) {
			sameDayMeasurements = append(sameDayMeasurements, m)
		}
	}

	return sameDayMeasurements
}

func ParseTimestamp(timestamp string) (time.Time, error) {
	// Placeholder for timestamp parsing logic
	// This should parse the timestamp string and return a standardized format
	// For now, we will just return the input as is
	if timestamp == "" {
		return time.Time{}, fmt.Errorf("timestamp cannot be empty")
	}

	return time.Parse(time.RFC3339, timestamp)
}

func IsDameDay(t1, t2 time.Time) bool {
	loc := time.UTC
	t1Local := t1.In(loc)
	t2Local := t2.In(loc)

	// Create exact start of day
	start1 := time.Date(t1Local.Year(), t1Local.Month(), t1Local.Day(), 0, 0, 0, 0, loc)
	start2 := time.Date(t2Local.Year(), t2Local.Month(), t2Local.Day(), 0, 0, 0, 0, loc)

	return start1.Equal(start2)
}
