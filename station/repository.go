package station

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/spinframework/spin-go-sdk/kv"
)

const (
	AllStationsKey = "all_stations"
)

var (
	ErrKVStoreNotAvailable = errors.New("KV store not available")
)

type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type Repository interface {
	List(ctx context.Context, pagination *Pagination) (*StationCollection, error)
	CreateList(ctx context.Context, stations *StationCollection) error

	GetByID(ctx context.Context, id string) (*Station, error)
	Create(ctx context.Context, station *Station) error

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

func (r *SpinKVRepository) List(ctx context.Context, pagination *Pagination) (*StationCollection, error) {
	defer ctx.Done()

	jsonBlob, err := r.getKey(ctx, AllStationsKey)
	if err != nil {
		r.logger.Debug("No stations found in Spin KV, returning empty collection")
		return nil, err
	}

	var stationCollection StationCollection
	if err := json.Unmarshal(jsonBlob, &stationCollection); err != nil {
		r.logger.Error("Failed to unmarshal stations", "error", err)
		return nil, err
	}

	return &stationCollection, nil
}

func (r *SpinKVRepository) CreateList(ctx context.Context, stations *StationCollection) error {
	defer ctx.Done()

	if stations == nil {
		return errors.New("stations cannot be nil")
	}

	r.logger.Debug("Adding stations to Spin KV")
	jsonBlob, err := json.Marshal(stations)
	if err != nil {
		r.logger.Error("Failed to marshal stations", "error", err)
		return err
	}

	if err := r.setKey(ctx, AllStationsKey, jsonBlob); err != nil {
		r.logger.Error("Failed to add stations to Spin KV", "error", err)
		return err
	}

	// also store each station individually
	for _, station := range stations.Stations {
		if err := r.Create(ctx, &station); err != nil {
			r.logger.Error("Failed to add individual station to Spin KV", "id", station.UUID, "error", err)
			return err
		}
	}

	r.logger.Info("Stations added successfully to Spin KV")
	return nil
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

	if station.UUID == "" {
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

	if err := r.setKey(ctx, station.UUID, jsonBlob); err != nil {
		r.logger.Error("Failed to add station to Spin KV", "error", err)
		return err
	}

	r.logger.Debug("Station added to Spin KV", "id", station.UUID)
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
