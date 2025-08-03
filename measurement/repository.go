package measurement

import "context"

type Repository interface {
	GetTimeseries(ctx context.Context, measurementName string, period Period) (*Timeseries, error)
	AddTimeseries(ctx context.Context, timeseries *Timeseries) error

	AddMeasurement(ctx context.Context, measurement *Measurement) error
	// TODO: we should add pagination to this method
	GetMeasurements(ctx context.Context) ([]Measurement, error)

	// IsReady checks if the repository is ready for operations.
	IsReady() bool
	Close() error
}
