package service

import (
	"context"
	"net/url"
	"web_page_analyzer/internal/domain/adaptors"
	"web_page_analyzer/internal/domain/models"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type JobService struct {
	analyzer   adaptors.AnalyzerService
	jobStore   adaptors.JobRepository
	workerPool *WorkerPool
	log        *log.Logger
}

func NewJobService(analyzer adaptors.AnalyzerService, jobStore adaptors.JobRepository, log *log.Logger, numWorkers int) *JobService {
	s := &JobService{
		analyzer: analyzer,
		jobStore: jobStore,
		log:      log,
	}

	workerFunc := func(ctx context.Context, payload interface{}) (interface{}, error) {
		job := payload.(*models.Job)
		analysisResult, err := s.analyzer.Analyze(ctx, job.Result.BaseUrl.String())
		if err != nil {
			job.Status = models.JobStatusFailed
			s.jobStore.Set(job)
			return nil, err
		}

		job.Result = analysisResult
		job.Status = models.JobStatusCompleted
		s.jobStore.Set(job)
		return nil, nil
	}

	s.workerPool = NewWorkerPool(numWorkers, workerFunc)
	s.workerPool.Start(context.Background())

	return s
}

func (s *JobService) CreateJob(ctx context.Context, rawURL string) (*models.Job, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	jobID := uuid.New().String()
	job := &models.Job{
		ID:     jobID,
		Status: models.JobStatusPending,
		Result: &models.AnalysisResult{
			BaseUrl: parsedURL,
		},
	}
	s.jobStore.Set(job)

	s.workerPool.Submit(Job{ID: jobID, Payload: job})

	return job, nil
}
