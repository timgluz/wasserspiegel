package response

import (
	"fmt"
	"log/slog"
	"net/http"
)

var ErrNotFound = fmt.Errorf("request resource does not exist")

func NewNotFoundHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Warn("Resource not found", "path", r.URL.Path)
		RenderError(w, ErrNotFound, http.StatusNotFound)
	}
}
