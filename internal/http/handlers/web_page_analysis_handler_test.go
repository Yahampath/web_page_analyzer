package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"web_page_analyzer/internal/domain/models"
	"web_page_analyzer/internal/http/handlers"
	"web_page_analyzer/internal/platform/storage"
	"web_page_analyzer/internal/service"

	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockWebClient struct {
	mock.Mock
}

func (m *MockWebClient) Do(ctx context.Context, url string, method string) ([]byte, int, error) {
	args := m.Called(ctx, url, method)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]byte), args.Int(1), args.Error(2)
}

func TestWebPageAnalysisE2E(t *testing.T) {
	// Setup
	logger := log.New()
	mockWebClient := new(MockWebClient)
	jobStore := storage.NewInMemoryJobRepository()

	htmlContent := `
		<!DOCTYPE html>
		<html>
		<head><title>Test Page</title></head>
		<body>
			<h1>Welcome</h1>
			<a href="/internal">Internal Link</a>
			<form><input type="password"/></form>
		</body>
		</html>`

	mockWebClient.On("Do", mock.Anything, "http://example.com", http.MethodGet).Return([]byte(htmlContent), http.StatusOK, nil)

	analyzer := service.NewAnalyzer(logger, mockWebClient)
	jobService := service.NewJobService(analyzer, jobStore, logger, 2)
	analysisHandler := handlers.NewWebPageAnalysisHandler(jobService, logger)
	statusHandler := handlers.NewStatusHandler(jobStore, logger)
	resultHandler := handlers.NewResultHandler(jobStore, logger)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/analyze", analysisHandler.Handle)
		r.Get("/status/{job_id}", statusHandler.Handle)
		r.Get("/result/{job_id}", resultHandler.Handle)
	})

	// 1. POST /api/v1/analyze
	reqBody, _ := json.Marshal(handlers.WebPageAnalysisRequest{URL: "http://example.com"})
	req := httptest.NewRequest("POST", "/api/v1/analyze", bytes.NewBuffer(reqBody))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var analyzeResp handlers.WebPageAnalysisResponse
	err := json.Unmarshal(rr.Body.Bytes(), &analyzeResp)
	assert.NoError(t, err)
	assert.NotEmpty(t, analyzeResp.JobID)

	jobID := analyzeResp.JobID

	// 2. Poll for analysis to complete
	var statusResp handlers.StatusResponse
	completed := false
	for i := 0; i < 50; i++ {
		req = httptest.NewRequest("GET", "/api/v1/status/"+jobID, nil)
		rr = httptest.NewRecorder()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("job_id", jobID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		r.ServeHTTP(rr, req)

		err = json.Unmarshal(rr.Body.Bytes(), &statusResp)
		assert.NoError(t, err)

		if statusResp.Status == "completed" {
			completed = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.True(t, completed, "job did not complete in time")

	// 4. GET /api/v1/result/{job_id}
	req = httptest.NewRequest("GET", "/api/v1/result/"+jobID, nil)
	rr = httptest.NewRecorder()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("job_id", jobID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resultResp models.AnalysisResult
	err = json.Unmarshal(rr.Body.Bytes(), &resultResp)
	assert.NoError(t, err)

	assert.Equal(t, "HTML5", resultResp.HTMLVersion)
	assert.Equal(t, "Test Page", resultResp.Title)
	assert.Equal(t, 1, resultResp.Headings["h1"])
	assert.Equal(t, 1, resultResp.InternalLinks)
	assert.Equal(t, 0, resultResp.ExternalLinks)
	assert.True(t, resultResp.HasLoginForm)
}
