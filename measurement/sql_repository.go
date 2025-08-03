package measurement

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/spinframework/spin-go-sdk/v2/sqlite"
)

var (
	ErrDBNotAvailable = fmt.Errorf("SQLite DB is not available")
)

type SQLRepository struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewSpinSqliteDB(dbName string) (*sql.DB, error) {
	// Open the SQLite database
	db := sqlite.Open("dbName")
	// Check if the database is reachable
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping SQLite DB: %w", err)
	}

	return db, nil
}

func NewSqlRepository(db *sql.DB, logger *slog.Logger) (*SQLRepository, error) {
	if db == nil {
		logger.Error("SQL DB is not initialized")
		return nil, ErrDBNotAvailable
	}

	return &SQLRepository{
		db:     db,
		logger: logger,
	}, nil
}

func (r *SQLRepository) IsReady() bool {
	if r.logger == nil {
		fmt.Println("Logger of SQLRepository is not initialized")
		return false
	}

	if r.db == nil {
		r.logger.Error("SQLite DB is not initialized")
		return false
	}

	// TODO: check if tables exists

	return true
}

func (r *SQLRepository) Close() error {
	if r.db == nil {
		return fmt.Errorf("SQLite DB is not initialized")
	}

	if err := r.db.Close(); err != nil {
		r.logger.Error("Failed to close SQLite DB", "error", err)
		return err
	}

	r.logger.Info("SQLite DB closed successfully")
	return nil
}

func (r *SQLRepository) AddTimeseries(ctx context.Context, timeseries *Timeseries) error {
	defer ctx.Done()

	if timeseries == nil {
		r.logger.Error("Cannot add nil timeseries")
		return fmt.Errorf("timeseries cannot be nil")
	}

	// Ensure the measurement exists or create it
	ok, err := r.hasMeasurement(timeseries.Name)
	if err != nil {
		r.logger.Error("Failed to get measurement by name", "name", timeseries.Name, "error", err)
		return err
	}

	if !ok {
		if err := r.AddMeasurement(ctx, timeseries.Measurement); err != nil {
			r.logger.Error("Failed to add measurement", "measurement", timeseries.Measurement, "error", err)
			return err
		}
		r.logger.Info("Measurement added", "measurement", timeseries.Measurement)
	}

	// Retrieve the measurement ID
	measurement, err := r.getMeasurementByName(timeseries.Name)
	if err != nil {
		r.logger.Error("Failed to get measurement by name", "name", timeseries.Name, "error", err)
		return err
	}

	if measurement == nil {
		r.logger.Error("Measurement not found after adding", "name", timeseries.Name)
		return fmt.Errorf("measurement not found after adding: %s", timeseries.Name)
	}

	// Insert samples into the database
	for _, sample := range timeseries.Samples {
		if err := r.addSample(measurement.ID, sample); err != nil {
			r.logger.Error("Failed to add sample", "sample", sample, "error", err)
			return err
		}
		r.logger.Info("Sample added", "sample", sample)
	}

	r.logger.Info("Timeseries added successfully", "name", timeseries.Name)
	return nil
}

// GetTimeseries retrieves a timeseries for a given measurement name and time range.
func (r *SQLRepository) GetTimeseries(ctx context.Context, measurementName string, period Period) (*Timeseries, error) {
	measurement, err := r.getMeasurementByName(measurementName)
	if err != nil {
		r.logger.Error("Failed to get measurement by name", "name", measurementName, "error", err)
		return nil, err
	}
	if measurement == nil {
		r.logger.Info("Measurement not found", "name", measurementName)
		return nil, nil // Measurement not found
	}

	samples, err := r.getSampleSpanByMeasurementID(measurement.ID)
	if err != nil {
		r.logger.Error("Failed to get samples for measurement", "measurement_id", measurement.ID, "error", err)
		return nil, err
	}

	timeseries := &Timeseries{
		Name:        measurement.Name,
		Samples:     samples,
		Start:       period.Start,
		End:         period.End,
		Measurement: measurement,
	}

	r.logger.Info("Timeseries retrieved", "measurement_name", measurement.Name, "start", period.Start, "end", period.End)
	return timeseries, nil
}

// hasMeasurement checks if a measurement with the given name exists.
func (r *SQLRepository) hasMeasurement(name string) (bool, error) {
	query := `SELECT COUNT(*) FROM measurements WHERE name = ?`
	row := r.db.QueryRow(query, name)

	var count int
	if err := row.Scan(&count); err != nil {
		r.logger.Error("Failed to scan measurement count", "name", name, "error", err)
		return false, err
	}

	exists := count > 0
	r.logger.Info("Measurement existence checked", "name", name, "exists", exists)
	return exists, nil
}

// addMeasurement adds a new measurement to the database.
func (r *SQLRepository) AddMeasurement(ctx context.Context, measurement *Measurement) error {
	defer ctx.Done()

	if measurement == nil {
		r.logger.Error("Cannot add nil measurement")
		return fmt.Errorf("measurement cannot be nil")
	}
	query := `INSERT INTO measurements (name, unit, description) VALUES (?, ?, ?)`
	_, err := r.db.Exec(query, measurement.Name, measurement.Unit, measurement.Description)
	if err != nil {
		r.logger.Error("Failed to insert measurement", "measurement", measurement, "error", err)
		return err
	}
	r.logger.Info("Measurement added to database", "measurement", measurement)
	return nil
}

// GetMeasurements retrieves all measurements from the database.
func (r *SQLRepository) GetMeasurements(ctx context.Context) ([]Measurement, error) {
	defer ctx.Done()

	query := `SELECT id, name, unit FROM measurements ORDER BY name`
	rows, err := r.db.Query(query)
	if err != nil {
		r.logger.Error("Failed to query measurements", "error", err)
		return nil, err
	}
	defer rows.Close()

	var measurements []Measurement
	for rows.Next() {
		var measurement Measurement
		if err := rows.Scan(&measurement.ID, &measurement.Name, &measurement.Unit); err != nil {
			r.logger.Error("Failed to scan measurement row", "error", err)
			return nil, err
		}
		measurements = append(measurements, measurement)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error occurred during row iteration", "error", err)
		return nil, err
	}

	r.logger.Info("Measurements retrieved successfully", "count", len(measurements))
	return measurements, nil
}

// GetMeasurementByID retrieves a measurement by its ID.
func (r *SQLRepository) getMeasurementByName(id string) (*Measurement, error) {
	query := `SELECT id, name, unit FROM measurements WHERE id = ?`
	row := r.db.QueryRow(query, id)

	var measurement Measurement
	if err := row.Scan(&measurement.ID, &measurement.Name, &measurement.Unit); err != nil {
		if err == sql.ErrNoRows {
			r.logger.Info("Measurement not found", "id", id)
			return nil, nil // Measurement not found
		}
		r.logger.Error("Failed to scan measurement row", "error", err)
		return nil, err
	}
	r.logger.Info("Measurement retrieved", "id", measurement.ID, "name", measurement.Name)
	return &measurement, nil
}

func (r *SQLRepository) hasSample(measurementID int64, timestamp Epoch) (bool, error) {
	query := `SELECT COUNT(*) FROM samples WHERE measurement_id = ? AND timestamp = ?`
	row := r.db.QueryRow(query, measurementID, timestamp)

	var count int
	if err := row.Scan(&count); err != nil {
		r.logger.Error("Failed to scan sample count", "measurement_id", measurementID, "timestamp", timestamp, "error", err)
		return false, err
	}

	exists := count > 0
	r.logger.Info("Sample existence checked", "measurement_id", measurementID, "timestamp", timestamp, "exists", exists)
	return exists, nil
}

// addSample adds a sample to the database.
func (r *SQLRepository) addSample(measurementID int64, sample Sample) error {
	ok, err := r.hasSample(measurementID, sample.Timestamp)
	if err != nil {
		r.logger.Error("Failed to check if sample exists", "measurement_id", measurementID, "timestamp", sample.Timestamp, "error", err)
		return err
	}

	if ok {
		r.logger.Info("skip: Sample already exists", "measurement_id", measurementID, "timestamp", sample.Timestamp)
		return nil // Sample already exists, no need to insert
	}

	query := `INSERT INTO samples (measurement_id, value, timestamp) VALUES (?, ?, ?)`
	if _, err := r.db.Exec(query, measurementID, sample.Value, sample.Timestamp); err != nil {
		r.logger.Error("Failed to insert sample", "sample", sample, "error", err)
		return err
	}

	r.logger.Info("Sample added to database", "sample", sample)
	return nil
}

func (r *SQLRepository) getSampleSpanByMeasurementID(measurementID int64) ([]Sample, error) {
	query := `
SELECT id, measurement_id, value, timestamp
FROM samples
WHERE measurement_id = ?
	AND timestamp >= ? AND timestamp <= ?
ORDER BY timestamp ASC`

	rows, err := r.db.Query(query, measurementID)
	if err != nil {
		r.logger.Error("Failed to query samples", "error", err)
		return nil, err
	}
	defer rows.Close()

	var samples []Sample
	for rows.Next() {
		var sample Sample
		if err := rows.Scan(&sample.ID, &sample.MeasurementID, &sample.Value, &sample.Timestamp); err != nil {
			r.logger.Error("Failed to scan sample row", "error", err)
			return nil, err
		}
		samples = append(samples, sample)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error occurred during row iteration", "error", err)
		return nil, err
	}

	return samples, nil
}
