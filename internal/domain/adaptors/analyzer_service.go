package adaptors

import (
	"context"
	"web_page_analyzer/internal/domain/models"
)

// AnalyzerService defines the interface for the web page analysis service.
// This allows the application layer to depend on an abstraction for the
// core domain logic.
type AnalyzerService interface {
	Analyze(ctx context.Context, userURL string) (*models.AnalysisResult, error)
}
