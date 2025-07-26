package station

import (
	"context"
	"log/slog"
)

type Seeder interface {
	Seed(ctx context.Context, provider Provider, repository Repository) error
}

type ProviderSeeder struct {
	logger *slog.Logger
}

func NewProviderSeeder(logger *slog.Logger) *ProviderSeeder {
	return &ProviderSeeder{
		logger: logger,
	}
}

func (s *ProviderSeeder) Seed(ctx context.Context, provider Provider, repository Repository) error {
	if !provider.IsReady() {
		return ErrProviderNotReady
	}

	if !repository.IsReady() {
		return ErrKVStoreNotAvailable
	}

	collection, err := provider.GetStations(ctx)
	if err != nil {
		return err
	}

	if len(collection.Stations) == 0 {
		s.logger.Info("No stations found to seed")
		return nil
	}

	s.logger.Info("Seeding stations", "count", len(collection.Stations))
	for i := range collection.Stations {
		station := &collection.Stations[i]
		if repository.Has(ctx, station.ID) {
			if err := repository.Delete(ctx, station.ID); err != nil {
				s.logger.Error("Failed to delete existing station", "id", station.ID, "error", err)
				return err
			}
		}

		if err := repository.Create(ctx, station); err != nil {
			s.logger.Error("Failed to create station in repository", "id", station.ID, "error", err)
			return err
		}
		s.logger.Info("Station seeded successfully", "id", station.ID)
	}
	s.logger.Info("Seeding completed successfully")
	return nil
}
