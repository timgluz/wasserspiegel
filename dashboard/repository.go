package dashboard

import "context"

const (
	DefaultLimit  = 100
	DefaultOffset = 0
)

type Repository interface {
	List(ctx context.Context, offset int, limit int) (*Collection, error)

	GetByID(ctx context.Context, id string) (*Dashboard, error)
	Add(ctx context.Context, dashboard *Dashboard) error
	Update(ctx context.Context, dashboard *Dashboard) error
	Delete(ctx context.Context, id string) error

	IsReady() bool
	Close() error
}

func StreamDashboards(ctx context.Context, repo Repository, offset, limit int) (<-chan ListItem, <-chan error) {
	outCh := make(chan ListItem)
	errCh := make(chan error, 1)

	go func() {
		defer close(outCh)
		defer close(errCh)

		if limit <= 0 {
			limit = DefaultLimit
		}
		if offset < 0 {
			offset = DefaultOffset
		}

		for {
			dashboardCollection, err := repo.List(ctx, offset, limit)
			if err != nil {
				errCh <- err
				return
			}

			if len(dashboardCollection.Items) == 0 {
				return
			}

			for _, item := range dashboardCollection.Items {
				select {
				case outCh <- item:
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				}
			}

			offset += limit
		}
	}()

	return outCh, errCh
}
