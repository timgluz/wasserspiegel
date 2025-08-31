package dashboard

import "context"

type Repository interface {
	List(ctx context.Context, offset int, limit int) ([]*Dashboard, error)

	GetByID(ctx context.Context, id string) (*Dashboard, error)
	Add(ctx context.Context, dashboard *Dashboard) error
	Update(ctx context.Context, dashboard *Dashboard) error
	Delete(ctx context.Context, id string) error

	IsReady() bool
	Close() error
}
