package response

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	JSONContentType = "application/json"
	HTMLContentType = "text/html"
)

func RenderFatal(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", JSONContentType)

	jsonError := fmt.Sprintf(`{"error": "%s"}`, err.Error())
	http.Error(w, jsonError, http.StatusInternalServerError)
}

func RenderError(w http.ResponseWriter, err error, statusCode int) {
	w.Header().Set("Content-Type", JSONContentType)

	jsonError := fmt.Sprintf(`{"error": "%s"}`, err.Error())
	http.Error(w, jsonError, statusCode)
}

func RenderSuccess(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", JSONContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func RenderJSONResponse(w http.ResponseWriter, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		RenderFatal(w, fmt.Errorf("failed to marshal data: %w", err))
		return
	}

	w.Header().Set("Content-Type", JSONContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jsonData)
}
