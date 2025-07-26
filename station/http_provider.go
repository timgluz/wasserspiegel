package station

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

type HTTPProvider struct {
	client *http.Client
	logger *slog.Logger
}

func NewHTTPProvider(client *http.Client, logger *slog.Logger) *HTTPProvider {
	return &HTTPProvider{
		client: client,
		logger: logger,
	}
}

func (p *HTTPProvider) IsReady() bool {
	if p.logger == nil {
		fmt.Println("Logger of HTTPProvider is not initialized")
		return false
	}

	if p.client == nil {
		p.logger.Error("HTTP client is not set for HTTPProvider")
		return false
	}

	return true
}

func (p *HTTPProvider) RetrieveContent(ctx context.Context, url string) (io.Reader, error) {
	defer ctx.Done()

	if !p.IsReady() {
		return nil, fmt.Errorf("HTTPProvider is not ready")
	}

	req, err := p.client.Get(url)
	if err != nil {
		return nil, err
	}

	defer func(req *http.Response) {
		if err := req.Body.Close(); err != nil {
			p.logger.Error("Failed to close response body", "url", url, "error", err)
		}
	}(req)

	if req.StatusCode != http.StatusOK {
		if req.StatusCode == http.StatusNotFound {
			return nil, ErrResourceNotFound
		}

		return nil, fmt.Errorf("failed to fetch stations: %s", req.Status)
	}

	content, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read content from URL %s: %w", url, err)
	}

	if len(content) == 0 {
		p.logger.Warn("No content received from URL", "url", url)
		return nil, ErrNoContent
	}

	p.logger.Info("Content retrieved successfully", "url", url, "length", len(content))
	return bytes.NewReader(content), nil
}
