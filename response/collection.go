package response

type CollectionResponse[T any] struct {
	Items      []T         `json:"items"`
	Total      int         `json:"total"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

func NewCollectionResponse[T any](items []T, pagination *Pagination) CollectionResponse[T] {
	return CollectionResponse[T]{
		Items:      items,
		Total:      len(items),
		Pagination: pagination,
	}
}
