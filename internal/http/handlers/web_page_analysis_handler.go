package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"web_page_analyzer/internal/pkg/contextkeys"
	"web_page_analyzer/internal/pkg/errors"
	"web_page_analyzer/internal/service"

	log "github.com/sirupsen/logrus"
)

type WebPageAnalysisHandler struct {
	jobService *service.JobService
	metrics    struct{}
	log        *log.Logger
}

type WebPageAnalysisRequest struct {
	URL string `json:"url"`
}

type WebPageAnalysisResponse struct {
	JobID string `json:"job_id"`
}

func (r *WebPageAnalysisRequest) Validate() error {

	if r.URL == "" {
		return errors.New("url is empty")
	}

	baseURL, err := url.Parse(r.URL)
	if err != nil {
		return errors.Wrap(err, `failed to parse url`)
	}

	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return errors.New("url is invalid")
	}

	return nil
}

func NewWebPageAnalysisHandler(jobService *service.JobService, log *log.Logger) *WebPageAnalysisHandler {
	return &WebPageAnalysisHandler{
		jobService: jobService,
		metrics:    struct{}{},
		log:        log,
	}
}

func (h *WebPageAnalysisHandler) Handle(w http.ResponseWriter, r *http.Request) {
	h.log.Debug(`analyze web page handler called`)

	var request WebPageAnalysisRequest
	reqID, _ := r.Context().Value(contextkeys.CtxKeyRequestID{}).(string)

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.log.WithError(err).Error(`failed to decode request body`)
		SendErrorResponse(w, http.StatusBadRequest, "invalid request body", "invalid_request", reqID)
		return
	}

	if err := request.Validate(); err != nil {
		h.log.WithError(err).Error(`failed to validate request body`)
		SendErrorResponse(w, http.StatusBadRequest, err.Error(), "validation_failed", reqID)
		return
	}

	job, err := h.jobService.CreateJob(r.Context(), request.URL)
	if err != nil {
		h.log.WithError(err).Error("failed to create job")
		SendErrorResponse(w, http.StatusInternalServerError, "failed to create job", "job_creation_failed", reqID)
		return
	}

	response := WebPageAnalysisResponse{
		JobID: job.ID,
	}

	w.Header().Set(`Content-Type`, `application/json`)
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.WithError(err).Error(`failed to encode response`)
	}
}
