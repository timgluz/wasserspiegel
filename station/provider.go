package station

import (
	"context"
	"fmt"
)

var (
	ErrBaseURLNotSet    = fmt.Errorf("base URL not set")
	ErrHTTPClientNotSet = fmt.Errorf("HTTP client not set")
	ErrNoContent        = fmt.Errorf("no content available")
	ErrInvalidStationID = fmt.Errorf("invalid station ID provided")
	ErrUnmarshalFailed  = fmt.Errorf("failed to unmarshal content")
	ErrResourceNotFound = fmt.Errorf("resource not found")
	ErrProviderNotReady = fmt.Errorf("provider is not ready")
)

type Provider interface {
	GetStations(ctx context.Context) (*StationCollection, error)
	GetStation(ctx context.Context, id string) (*Station, error)
	GetStationWaterLevel(ctx context.Context, id string) (*WaterLevelCollection, error)
	IsReady() bool
}
