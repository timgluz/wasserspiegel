package station

import (
	"context"
	"fmt"
)

const (
	AllStationsKey = "all_stations"
	DefaultLimit   = 100
	DefaultOffset  = 0
)

type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// fix: return slice of pointers to Station, like Dashboard repo
type Repository interface {
	List(ctx context.Context, offset int, limit int) (*StationCollection, error)

	Has(ctx context.Context, id string) bool
	GetByID(ctx context.Context, id string) (*Station, error)
	Create(ctx context.Context, station *Station) error
	Delete(ctx context.Context, id string) error

	IsReady() bool
	Close() error
}

// StreamStations returns a channel that yields all stations from the repository,
// handling pagination internally. It also returns a channel for errors.
// The function respects the provided context for cancellation.
func StreamStations(ctx context.Context, repo Repository, offset, limit int) (<-chan Station, <-chan error) {
	out := make(chan Station)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		if limit <= 0 {
			limit = DefaultLimit
		}
		if offset < 0 {
			offset = DefaultOffset
		}

		for {
			collection, err := repo.List(ctx, offset, limit)
			if err != nil {
				errc <- err
				return
			}
			if len(collection.Stations) == 0 {
				fmt.Println("No more stations to process, ending stream.")
				return
			}

			for _, station := range collection.Stations {
				select {
				case out <- station:
				case <-ctx.Done():
					errc <- ctx.Err()
					return
				}
			}

			offset += limit
		}
	}()

	return out, errc
}
