package station

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type APIRepository struct {
	client *http.Client

	baseURL string
	apiKey  string
}

func NewAPIRepository(client *http.Client, baseURL, apiKey string) *APIRepository {
	return &APIRepository{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

func (r *APIRepository) List(ctx context.Context, offset int, limit int) (*StationCollection, error) {
	defer ctx.Done()

	resourceURL := r.baseURL + "/stations"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Accept", "application/json")

	q := req.URL.Query()
	if limit > 0 {
		q.Add("limit", fmt.Sprintf("%d", limit))
	}
	if offset > 0 {
		q.Add("offset", fmt.Sprintf("%d", offset))
	}
	req.URL.RawQuery = q.Encode()

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(resp *http.Response) {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %v\n", err)
		}
	}(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	var stations StationCollection
	if err := json.NewDecoder(resp.Body).Decode(&stations); err != nil {
		return nil, err
	}

	return &stations, nil
}

func (r *APIRepository) Has(ctx context.Context, id string) bool {
	defer ctx.Done()

	station, err := r.GetByID(ctx, id)
	if err != nil || station == nil {
		return false
	}

	return true
}

func (r *APIRepository) GetByID(ctx context.Context, id string) (*Station, error) {
	defer ctx.Done()

	resourceURL := fmt.Sprintf("%s/stations/%s", r.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(resp *http.Response) {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %v\n", err)
		}
	}(resp)

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	var station Station
	if err := json.NewDecoder(resp.Body).Decode(&station); err != nil {
		return nil, err
	}

	return &station, nil
}

// TODO: Implement Create and Delete methods if the API supports them.
func (r *APIRepository) Create(ctx context.Context, station *Station) error {
	defer ctx.Done()

	return fmt.Errorf("Create operation is not supported in APIRepository")
}

func (r *APIRepository) Delete(ctx context.Context, id string) error {
	defer ctx.Done()

	return fmt.Errorf("Delete operation is not supported in APIRepository")
}

func (r *APIRepository) IsReady() bool {
	if r.client == nil || r.baseURL == "" {
		return false
	}
	return true
}

func (r *APIRepository) Close() error {
	if r.client != nil {
		r.client = nil
	}

	return nil
}
