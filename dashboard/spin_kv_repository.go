//go:build tinygo || wasm

package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/spinframework/spin-go-sdk/v2/kv"
	"github.com/timgluz/wasserspiegel/measurement"
	"github.com/timgluz/wasserspiegel/response"
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
func (r *SpinKVRepository) List(ctx context.Context, offset int, limit int) (*Collection, error) {
	defer ctx.Done()

	if !r.IsReady() {
		return nil, ErrKVStoreNotAvailable
	}

	if limit <= 0 {
		limit = 100 // Default limit
	}
	if offset < 0 {
		offset = 0 // Default offset
	}

	keys, err := r.db.GetKeys()
	if err != nil {
		r.logger.Error("Failed to retrieve keys from Spin KV store", "error", err)
		return nil, err
	}

	if offset >= len(keys) {
		r.logger.Debug("Offset exceeds total number of dashboards", "offset", offset, "total", len(keys))
		return nil, nil
	}

	until := offset + limit
	if until > len(keys) {
		until = len(keys)
	}

	var dashboards []Dashboard
	for _, key := range keys[offset:until] {
		dashboard, err := r.GetByID(ctx, key)
		if err != nil {
			r.logger.Error("Failed to get dashboard by key", "key", key, "error", err)
			continue
		}

		if dashboard == nil {
			r.logger.Warn("Dashboard not found for key", "key", key)
			continue
		}

		dashboards = append(dashboards, *dashboard)
	}

	r.logger.Debug("Listed dashboards from Spin KV store", "count", len(dashboards), "offset", offset, "limit", limit)
	pagination := response.Pagination{
		Limit:  limit,
		Offset: offset,
		Total:  len(keys),
	}
	collection := NewDashboardListCollection(dashboards, pagination)
	return collection, nil
}

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

	if dashboard.ID == "" {
		r.logger.Debug("Dashboard ID is empty, generating a new ID")
		if id, err := GenerateDashboardID(dashboard); err != nil {
			r.logger.Error("Failed to generate dashboard ID", "error", err)
			return fmt.Errorf("failed to generate dashboard ID: %w", err)
		} else {
			dashboard.ID = id
		}
	}

	dashboard.CreatedAt = measurement.CurrentUnix()
	dashboard.UpdatedAt = dashboard.CreatedAt

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

	dashboard.UpdatedAt = measurement.CurrentUnix()

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
	if id == "" {
		r.logger.Warn("Cannot delete dashboard: ID is empty")
		return nil
	}

	if !r.IsReady() {
		return ErrKVStoreNotAvailable
	}

	if err := r.db.Delete(id); err != nil {
		r.logger.Error("Failed to delete dashboard from Spin KV store", "id", id, "error", err)
		return fmt.Errorf("failed to delete dashboard with ID %s: %w", id, err)
	}

	r.logger.Debug("Deleted dashboard from Spin KV store", "id", id)
	return nil
}
