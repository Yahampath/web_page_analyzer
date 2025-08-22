package storage

import "web_page_analyzer/internal/domain/models"

type JobStore interface {
	Get(id string) (*models.Job, bool)
	Set(job *models.Job)
}
