package handlers

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type ErrorResponse struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	Code    int    `json:"code"`
}

func sendError(w http.ResponseWriter, message string, err error, code int) {
	log.WithFields(log.Fields{
		"error": err,
		"code": code,
	}).Error(message)

	response := ErrorResponse{
		Message: message,
		Error:   err.Error(),
		Code:    code,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(response)
}