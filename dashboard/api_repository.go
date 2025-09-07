package dashboard

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

func NewAPIRepository(httpClient *http.Client, baseURL, apiKey string) *APIRepository {
	return &APIRepository{
		client:  httpClient,
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// List fetches a list of dashboards from the external API.
func (r *APIRepository) List(ctx context.Context, offset int, limit int) (*Collection, error) {
	defer ctx.Done()

	resourceURL := r.baseURL + "/dashboards"
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

	var dashboards Collection
	if err := json.NewDecoder(resp.Body).Decode(&dashboards); err != nil {
		return nil, err
	}

	return &dashboards, nil
}

// GetByID fetches a single dashboard by its ID from the external API.
func (r *APIRepository) GetByID(ctx context.Context, id string) (*Dashboard, error) {
	// Implementation to call the external API and fetch a dashboard by ID.
	return nil, nil
}

// Add creates a new dashboard via the external API.
func (r *APIRepository) Add(ctx context.Context, dashboard *Dashboard) error {
	// Implementation to call the external API and add a new dashboard.
	return nil
}

// Update modifies an existing dashboard via the external API.
func (r *APIRepository) Update(ctx context.Context, dashboard *Dashboard) error {
	// Implementation to call the external API and update an existing dashboard.
	return nil
}

// Delete removes a dashboard by its ID via the external API.
func (r *APIRepository) Delete(ctx context.Context, id string) error {
	// Implementation to call the external API and delete a dashboard by ID.
	return nil
}

func (r *APIRepository) IsReady() bool {
	return r.client != nil
}

func (r *APIRepository) Close() error {
	if r.client != nil {
		r.client = nil
	}

	return nil
}
