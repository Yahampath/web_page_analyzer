package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"web_page_analyzer/internal/pkg/errors"

	"github.com/sirupsen/logrus/hooks/test"
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

func TestAnalyzer_Analyze_Success(t *testing.T) {
	// Setup
	logger, hook := test.NewNullLogger()
	mockWebClient := new(MockWebClient)

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

	analyzer := &Analyzer{
		log:         logger,
		webClient:   mockWebClient,
		linkChecker: func(ctx context.Context, links []map[string]interface{}) int { return 0 }, // Mocked
	}

	// Execute
	result, err := analyzer.Analyze(context.Background(), "http://example.com")

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	mockWebClient.AssertExpectations(t)

	assert.Equal(t, "HTML5", result.HTMLVersion)
	assert.Equal(t, "Test Page", result.Title)
	assert.Equal(t, 1, result.Headings["h1"])
	assert.Equal(t, 1, result.InternalLinks)
	assert.Equal(t, 0, result.ExternalLinks)
	assert.True(t, result.HasLoginForm)
	assert.Equal(t, 0, result.InaccessibleLinks)

	// Assert logs
	assert.True(t, len(hook.Entries) > 0)
	assert.Equal(t, "starting web page analysis", hook.Entries[0].Message)
	assert.Equal(t, "finished web page analysis", hook.LastEntry().Message)
}

func TestAnalyzer_Analyze_PrerequisiteFailure(t *testing.T) {
	// Setup
	logger, hook := test.NewNullLogger()
	mockWebClient := new(MockWebClient)
	mockWebClient.On("Do", mock.Anything, "http://example.com", http.MethodGet).Return(nil, 0, errors.New("mocked error"))

	analyzer := NewAnalyzer(logger, mockWebClient)

	// Execute
	_, err := analyzer.Analyze(context.Background(), "http://example.com")

	// Assertions
	assert.Error(t, err)
	mockWebClient.AssertExpectations(t)

	// Assert logs
	assert.True(t, len(hook.Entries) > 0)
	logEntry := hook.LastEntry()
	assert.Equal(t, "failed to complete prerequisite tasks", logEntry.Message)
	assert.Contains(t, err.Error(), "mocked error")
}

func TestCheckLinksAccessibilityWithRealClient(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/good" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	links := []map[string]interface{}{
		{"url": server.URL + "/good", "isInternal": true},
		{"url": server.URL + "/bad", "isInternal": true},
		{"url": "http://invalid-url-that-will-fail.com", "isInternal": false},
	}

	inaccessibleCount := checkLinksAccessibility(context.Background(), links)

	assert.Equal(t, 2, inaccessibleCount)
}
