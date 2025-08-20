package storage

import (
	"sync"
	"web_page_analyzer/internal/domain/models"
)

type InMemoryJobStore struct {
	jobs map[string]*models.Job
	mu   sync.RWMutex
}

func NewInMemoryJobStore() *InMemoryJobStore {
	return &InMemoryJobStore{
		jobs: make(map[string]*models.Job),
	}
}

func (s *InMemoryJobStore) Get(id string) (*models.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, found := s.jobs[id]
	return job, found
}

func (s *InMemoryJobStore) Set(job *models.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
}
