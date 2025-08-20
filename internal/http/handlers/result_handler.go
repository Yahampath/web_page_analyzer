package handlers

import (
	"encoding/json"
	"net/http"
	"web_page_analyzer/internal/domain/models"
	"web_page_analyzer/internal/platform/storage"
	"web_page_analyzer/internal/pkg/contextkeys"

	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"
)

type ResultHandler struct {
	jobStore *storage.InMemoryJobStore
	log      *log.Logger
}

func NewResultHandler(jobStore *storage.InMemoryJobStore, log *log.Logger) *ResultHandler {
	return &ResultHandler{
		jobStore: jobStore,
		log:      log,
	}
}

func (h *ResultHandler) Handle(w http.ResponseWriter, r *http.Request) {
	reqID, _ := r.Context().Value(contextkeys.CtxKeyRequestID{}).(string)
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" {
		SendErrorResponse(w, http.StatusBadRequest, "job_id is required", "validation_failed", reqID)
		return
	}

	job, found := h.jobStore.Get(jobID)
	if !found {
		SendErrorResponse(w, http.StatusNotFound, "job not found", "not_found", reqID)
		return
	}

	if job.Status != models.JobStatusCompleted {
		SendErrorResponse(w, http.StatusBadRequest, "job is not complete", "job_not_complete", reqID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(job.Result); err != nil {
		h.log.WithError(err).Error("failed to encode response")
	}
}
