package response

import (
	"net/http"
	"strconv"
)

const (
	DefaultPaginationOffset = 0
	DefaultPaginationLimit  = 50
)

type Pagination struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Total  int `json:"total"`
}

func NewPagination(offset, limit, total int) Pagination {
	return Pagination{
		Offset: offset,
		Limit:  limit,
		Total:  total,
	}
}

// NewPaginationFromRequest creates a Pagination object from the request parameters.
// it silently defaults to DefaultPaginationOffset and DefaultPaginationLimit if the parameters are not provided or invalid.
func NewPaginationFromRequest(r *http.Request) Pagination {
	limit := DefaultPaginationLimit
	offset := DefaultPaginationOffset

	if offsetParam := r.URL.Query().Get("offset"); offsetParam != "" {
		val, err := strconv.Atoi(offsetParam)
		if err == nil {
			offset = val
		}
	}

	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		val, err := strconv.Atoi(limitParam)
		if err == nil {
			limit = val
		}
	}

	return NewPagination(offset, limit, 0)
}
