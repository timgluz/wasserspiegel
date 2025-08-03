package response

type PostResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"` // Use 'any' for flexible data types
}

func NewPostResponse(success bool, message string, data any) PostResponse {
	return PostResponse{
		Success: success,
		Message: message,
		Data:    data,
	}
}
