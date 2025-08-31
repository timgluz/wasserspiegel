package response

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"` // Use 'any' for flexible data types
}

func NewPostResponse(success bool, message string, data any) Response {
	return Response{
		Success: success,
		Message: message,
		Data:    data,
	}
}

func NewSuccessResponse(message string, data any) Response {
	return Response{
		Success: true,
		Message: message,
		Data:    data,
	}
}

type APIDocumentation struct {
	Title string `json:"title,omitempty"`
	Text  string `json:"text"`
}

func NewAPIDocumentationResponse(title, text string) Response {
	return Response{
		Success: true,
		Data:    APIDocumentation{Text: text},
	}
}
