package service

import (
	"context"
	"sync"
)

type Job struct {
	ID      int
	Payload interface{}
}

type Result struct {
	JobID int
	Value interface{}
	Err   error
}

type WorkerPool struct {
	numWorkers int
	jobs       chan Job
	results    chan Result
	wg         sync.WaitGroup
	doWork     func(ctx context.Context, payload interface{}) (interface{}, error)
}

func NewWorkerPool(numWorkers int, doWork func(ctx context.Context, payload interface{}) (interface{}, error)) *WorkerPool {
	return &WorkerPool{
		numWorkers: numWorkers,
		jobs:       make(chan Job),
		results:    make(chan Result),
		doWork:     doWork,
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx)
	}

	go func() {
		wp.wg.Wait()
		close(wp.results)
	}()
}

func (wp *WorkerPool) worker(ctx context.Context) {
	defer wp.wg.Done()
	for {
		select {
		case job, ok := <-wp.jobs:
			if !ok {
				return
			}
			value, err := wp.doWork(ctx, job.Payload)
			wp.results <- Result{
				JobID: job.ID,
				Value: value,
				Err:   err,
			}
		case <-ctx.Done():
			return
		}
	}
}

func (wp *WorkerPool) Submit(job Job) {
	wp.jobs <- job
}

func (wp *WorkerPool) Close() {
	close(wp.jobs)
}

func (wp *WorkerPool) Results() <-chan Result {
	return wp.results
}
