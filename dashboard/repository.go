package dashboard

import "context"

type Repository interface {
	GetByID(ctx context.Context, id string) (*Dashboard, error)
	Add(ctx context.Context, dashboard *Dashboard) error
	Update(ctx context.Context, dashboard *Dashboard) error
	Delete(ctx context.Context, id string) error
}
