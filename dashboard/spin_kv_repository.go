package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/spinframework/spin-go-sdk/v2/kv"
)

var (
	ErrKVStoreNotAvailable = fmt.Errorf("Spin KV store is not available")
)

type SpinKVRepository struct {
	db     *kv.Store
	logger *slog.Logger
}

func NewSpinKVRepository(storeName string, logger *slog.Logger) (*SpinKVRepository, error) {
	db, err := kv.OpenStore(storeName)
	if err != nil {
		logger.Error("Failed to open Spin KV store", "error", err)
		return nil, err
	}
	return &SpinKVRepository{
		db:     db,
		logger: logger,
	}, nil
}

// -- Component interface implementation --

func (r *SpinKVRepository) IsReady() bool {
	if r.logger == nil {
		r.logger.Error("Logger of SpinKVRepository is not initialized")
		return false
	}

	if r.db == nil {
		r.logger.Error("Spin KV store is not initialized")
		return false
	}

	return true
}

func (r *SpinKVRepository) Close() error {
	if r.db == nil {
		return nil // No action needed if db is not initialized
	}

	r.db.Close()
	r.logger.Info("Spin KV store closed successfully")
	return nil
}

// -- Repository interface implementation --
func (r *SpinKVRepository) GetByID(ctx context.Context, id string) (*Dashboard, error) {
	defer ctx.Done()

	if !r.IsReady() {
		return nil, ErrKVStoreNotAvailable
	}

	jsonBlob, err := r.db.Get(id)
	if err != nil {
		r.logger.Error("Failed to get dashboard by ID", "id", id, "error", err)
		return nil, err
	}

	if jsonBlob == nil {
		r.logger.Warn("Dashboard not found", "id", id)
		return nil, fmt.Errorf("dashboard with ID %s not found", id)
	}

	dashboard := &Dashboard{}
	if err := json.Unmarshal(jsonBlob, dashboard); err != nil {
		r.logger.Error("Failed to unmarshal dashboard JSON", "id", id, "error", err)
		return nil, fmt.Errorf("failed to unmarshal dashboard with ID %s: %w", id, err)
	}

	r.logger.Debug("Retrieved dashboard by ID", "id", id)
	return dashboard, nil
}

func (r *SpinKVRepository) Add(ctx context.Context, dashboard *Dashboard) error {
	defer ctx.Done()

	if !r.IsReady() {
		return ErrKVStoreNotAvailable
	}

	if dashboard == nil {
		return fmt.Errorf("dashboard cannot be nil")
	}

	jsonBlob, err := json.Marshal(dashboard)
	if err != nil {
		r.logger.Error("Failed to marshal dashboard", "error", err)
		return fmt.Errorf("failed to marshal dashboard: %w", err)
	}

	if err := r.db.Set(dashboard.ID, jsonBlob); err != nil {
		r.logger.Error("Failed to add dashboard to Spin KV store", "id", dashboard.ID, "error", err)
		return fmt.Errorf("failed to add dashboard with ID %s: %w", dashboard.ID, err)
	}

	r.logger.Debug("Added dashboard to Spin KV store", "id", dashboard.ID)
	return nil
}

func (r *SpinKVRepository) Update(ctx context.Context, dashboard *Dashboard) error {
	defer ctx.Done()

	if !r.IsReady() {
		return ErrKVStoreNotAvailable
	}

	if dashboard == nil {
		return fmt.Errorf("dashboard cannot be nil")
	}

	existingDashboard, err := r.GetByID(ctx, dashboard.ID)
	if err != nil {
		r.logger.Error("Failed to get existing dashboard for update", "id", dashboard.ID, "error", err)
		return fmt.Errorf("failed to get existing dashboard with ID %s: %w", dashboard.ID, err)
	}

	if existingDashboard != nil && existingDashboard.ID == dashboard.ID {
		r.logger.Debug("Dashboard already exists, updating it", "id", dashboard.ID)
		dashboard.Merge(existingDashboard) // Merge existing dashboard with the new one
	}

	jsonBlob, err := json.Marshal(dashboard)
	if err != nil {
		r.logger.Error("Failed to marshal dashboard", "error", err)
		return fmt.Errorf("failed to marshal dashboard: %w", err)
	}

	if err := r.db.Set(dashboard.ID, jsonBlob); err != nil {
		r.logger.Error("Failed to update dashboard in Spin KV store", "id", dashboard.ID, "error", err)
		return fmt.Errorf("failed to update dashboard with ID %s: %w", dashboard.ID, err)
	}

	r.logger.Debug("Updated dashboard in Spin KV store", "id", dashboard.ID)
	return nil
}

func (r *SpinKVRepository) Delete(ctx context.Context, id string) error {
	defer ctx.Done()

	if !r.IsReady() {
		return ErrKVStoreNotAvailable
	}

	if id == "" {
		return fmt.Errorf("dashboard ID cannot be empty")
	}

	if err := r.db.Delete(id); err != nil {
		r.logger.Error("Failed to delete dashboard from Spin KV store", "id", id, "error", err)
		return fmt.Errorf("failed to delete dashboard with ID %s: %w", id, err)
	}

	r.logger.Debug("Deleted dashboard from Spin KV store", "id", id)
	return nil
}
