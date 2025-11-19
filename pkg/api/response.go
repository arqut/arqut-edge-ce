package api

import "time"

// Map is a convenience type for map[string]interface{}
type Map map[string]interface{}

// Pagination contains pagination information
type Pagination struct {
	Page       int `json:"page"`
	PerPage    int `json:"perPage"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

// ApiResponseMeta contains metadata about the API response
type ApiResponseMeta struct {
	RequestID  string      `json:"requestId,omitempty"`
	Timestamp  *time.Time  `json:"timestamp,omitempty"`
	Ordering   *Map        `json:"ordering,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// ApiError represents an error in the API response
type ApiError struct {
	Code    int         `json:"code,omitempty"`
	Message string      `json:"message,omitempty"`
	Detail  interface{} `json:"detail,omitempty"`
}

// ApiResponse is the standard API response structure
type ApiResponse struct {
	Success bool             `json:"success"`
	Data    interface{}      `json:"data,omitempty"`
	Error   *ApiError        `json:"error,omitempty"`
	Meta    *ApiResponseMeta `json:"meta,omitempty"`
}
