package handlers

import (
	"encoding/json"
	"net/http"
)

// APIError represents a standard error response format.
type APIError struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id,omitempty"`
	} `json:"error"`
}

// SendErrorResponse sends a standardized JSON error response.
func SendErrorResponse(w http.ResponseWriter, code int, message, requestCode, requestID string) {
	errResp := APIError{}
	errResp.Error.Code = requestCode
	errResp.Error.Message = message
	errResp.Error.RequestID = requestID

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errResp)
}
