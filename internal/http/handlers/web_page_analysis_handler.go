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
	service *service.Analyzer
	metrics struct{}
	log     *log.Logger
}

type WebPageAnalysisRequest struct {
	URL string `json:"url"`
}

type WebPageAnalysisResponse struct {
	HTMLVersion       string         `json:"html_version"`
	Title             string         `json:"title"`
	Headings          map[string]int `json:"headings"`
	InternalLinks     int            `json:"internal_links"`
	ExternalLinks     int            `json:"external_links"`
	InaccessibleLinks int            `json:"inaccessible_links"`
	HasLoginForm      bool           `json:"has_login_form"`
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

func NewWebPageAnalysisHandler(service *service.Analyzer, log *log.Logger) *WebPageAnalysisHandler {
	return &WebPageAnalysisHandler{
		service: service,
		metrics: struct{}{},
		log:     log,
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

	result, err := h.service.Analyze(r.Context(), request.URL)
	if err != nil {
		h.log.WithError(err).Error("failed to analyze web page")
		SendErrorResponse(w, http.StatusInternalServerError, "failed to analyze web page", "analysis_failed", reqID)
		return
	}

	response := WebPageAnalysisResponse{
		HTMLVersion:       result.HTMLVersion,
		Title:             result.Title,
		Headings:          result.Headings,
		InternalLinks:     result.InternalLinks,
		ExternalLinks:     result.ExternalLinks,
		InaccessibleLinks: result.InaccessibleLinks,
		HasLoginForm:      result.HasLoginForm,
	}

	w.Header().Set(`Content-Type`, `application/json`)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.WithError(err).Error(`failed to encode response`)
		// We can't send another error response here because the header has already been written.
	}
}
