package storage

import (
	"sync"
	"web_page_analyzer/internal/domain/adaptors"
	"web_page_analyzer/internal/domain/models"
)

var _ adaptors.JobRepository = (*InMemoryJobRepository)(nil)

type InMemoryJobRepository struct {
	jobs map[string]*models.Job
	mu   sync.RWMutex
}

func NewInMemoryJobRepository() *InMemoryJobRepository {
	return &InMemoryJobRepository{
		jobs: make(map[string]*models.Job),
	}
}

func (s *InMemoryJobRepository) Get(id string) (*models.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, found := s.jobs[id]
	return job, found
}

func (s *InMemoryJobRepository) Set(job *models.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
}
