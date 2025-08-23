package adaptors

import "web_page_analyzer/internal/domain/models"

// JobRepository defines the interface for storing and retrieving Job aggregates.
// This is a core part of the domain layer, allowing the application to be
// independent of the storage implementation.
type JobRepository interface {
	Get(id string) (*models.Job, bool)
	Set(job *models.Job)
}
