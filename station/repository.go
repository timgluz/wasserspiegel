package station

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/spinframework/spin-go-sdk/v2/kv"
)

const (
	AllStationsKey = "all_stations"
	DefaultLimit   = 100
	DefaultOffset  = 0
)

var (
	ErrKVStoreNotAvailable = errors.New("KV store not available")
)

type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type Repository interface {
	List(ctx context.Context, offset int, limit int) (*StationCollection, error)

	Has(ctx context.Context, id string) bool
	GetByID(ctx context.Context, id string) (*Station, error)
	Create(ctx context.Context, station *Station) error
	Delete(ctx context.Context, id string) error

	IsReady() bool
	Close() error
}

type SpinKVRepository struct {
	db     *kv.Store
	logger *slog.Logger
}

func NewSpinKVRepository(storeName string, logger *slog.Logger) (Repository, error) {
	db, err := kv.OpenStore(storeName)
	if err != nil {
		logger.Error("Failed to open Spin KV store", "error", err)
		return nil, ErrKVStoreNotAvailable
	}

	return &SpinKVRepository{
		db:     db,
		logger: logger,
	}, nil
}

func (r *SpinKVRepository) IsReady() bool {
	if r.logger == nil {
		fmt.Println("Logger of SpinKVRepository  is not initialized")
		return false
	}

	if r.db == nil {
		r.logger.Error("Spin KV store is not initialized")
		return false
	}

	r.logger.Debug("Spin KV store is ready")
	return true
}

func (r *SpinKVRepository) List(ctx context.Context, offset int, limit int) (*StationCollection, error) {
	defer ctx.Done()

	keys, err := r.db.GetKeys()
	if err != nil {
		r.logger.Error("Failed to retrieve keys from Spin KV", "error", err)
		return nil, err
	}

	if len(keys) == 0 {
		r.logger.Info("No stations found in Spin KV")
	}

	if limit <= 0 || limit > len(keys) {
		limit = len(keys)
	}

	if offset < 0 || offset >= len(keys) {
		offset = 0
	}

	if offset+limit > len(keys) {
		limit = len(keys) - offset
	}

	r.logger.Debug("Listing stations from Spin KV", "limit", limit, "offset", offset)
	stations := make([]Station, 0, limit)
	for i := offset; i < offset+limit; i++ {
		if keys[i] == AllStationsKey {
			r.logger.Debug("Skipping AllStationsKey in listing")
			continue
		}

		station, err := r.GetByID(ctx, keys[i])
		if err != nil {
			r.logger.Error("Failed to get station by ID", "id", keys[i], "error", err)
			continue
		}

		if station == nil {
			r.logger.Warn("Got nil station for key", "key", keys[i])
			continue
		}

		stations = append(stations, *station)
	}

	return &StationCollection{Stations: stations}, nil
}

func (r *SpinKVRepository) Has(ctx context.Context, id string) bool {
	defer ctx.Done()

	if id == "" {
		r.logger.Warn("Empty station ID provided, cannot check existence")
		return false
	}

	if !r.IsReady() {
		r.logger.Error("Spin KV store is not ready, cannot check existence")
		return false
	}

	r.logger.Debug("Checking if station exists in Spin KV", "id", id)
	ok, err := r.db.Exists(id)
	if err != nil {
		r.logger.Error("Failed to check existence of station in Spin KV", "id", id, "error", err)
		return false
	}

	return ok
}

func (r *SpinKVRepository) GetByID(ctx context.Context, id string) (*Station, error) {
	defer ctx.Done()

	jsonBlob, err := r.getKey(ctx, id)
	if err != nil {
		return nil, err
	}

	station := &Station{}
	if err := json.Unmarshal(jsonBlob, station); err != nil {
		r.logger.Error("Failed to unmarshal station", "error", err)
		return nil, err
	}

	if station.ID == "" {
		r.logger.Warn("Unmarshalling returned empty station", "id", id)
	}

	return station, nil
}

func (r *SpinKVRepository) Create(ctx context.Context, station *Station) error {
	if station == nil {
		return errors.New("station cannot be nil")
	}

	jsonBlob, err := json.Marshal(station)
	if err != nil {
		r.logger.Error("Failed to marshal station", "error", err)
		return err
	}

	if err := r.setKey(ctx, station.ID, jsonBlob); err != nil {
		r.logger.Error("Failed to add station to Spin KV", "error", err)
		return err
	}

	r.logger.Debug("Station added to Spin KV", "id", station.ID)
	return nil
}

func (r *SpinKVRepository) Delete(ctx context.Context, id string) error {
	defer ctx.Done()
	if id == "" {
		return errors.New("station ID cannot be empty")
	}

	if !r.IsReady() {
		return ErrKVStoreNotAvailable
	}

	r.logger.Debug("Deleting station from Spin KV", "id", id)
	if err := r.db.Delete(id); err != nil {
		r.logger.Error("Failed to delete station from Spin KV", "id", id, "error", err)
		return err
	}
	r.logger.Info("Station deleted successfully from Spin KV", "id", id)
	return nil
}

func (r *SpinKVRepository) setKey(ctx context.Context, key string, data []byte) error {
	defer ctx.Done()

	if key == "" || data == nil {
		return errors.New("key and data cannot be empty")
	}

	if !r.IsReady() {
		return ErrKVStoreNotAvailable
	}

	r.logger.Debug("Storing blob in Spin KV", "key", key)
	if err := r.db.Set(key, data); err != nil {
		r.logger.Error("Failed to store blob in Spin KV", "error", err)
		return err
	}

	r.logger.Info("Blob stored successfully in Spin KV", "key", key)
	return nil
}

func (r *SpinKVRepository) getKey(ctx context.Context, key string) ([]byte, error) {
	defer ctx.Done()

	if key == "" {
		return nil, errors.New("key cannot be empty")
	}

	if !r.IsReady() {
		return nil, ErrKVStoreNotAvailable
	}

	r.logger.Debug("Retrieving blob from Spin KV", "key", key)
	data, err := r.db.Get(key)
	if err != nil {
		r.logger.Error("Failed to retrieve blob from Spin KV", "error", err)
		return nil, err
	}

	r.logger.Info("Blob retrieved successfully from Spin KV", "key", key)
	return data, nil
}

func (r *SpinKVRepository) Close() error {
	if r.db == nil {
		r.logger.Warn("Spin KV store is nil, nothing to close")
		return nil
	}

	r.db.Close() // Ensure the store is closed properly
	r.logger.Info("Spin KV store closed successfully")
	return nil
}
