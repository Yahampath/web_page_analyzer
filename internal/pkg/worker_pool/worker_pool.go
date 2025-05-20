package worker_pool

import (
	"context"
	"errors"
	"sync"

	log "github.com/sirupsen/logrus"
)

type TaskFunc func(ctx context.Context) (any, error)

// TaskResult holds the outcome of a finished task (its ID, result value, or error).
type TaskResult struct {
	ID     string
	Result any
	Err    error
}

// workItem is an internal wrapper for tasks submitted to the pool.
type workItem struct {
	id string
	fn TaskFunc
}

// WorkerPool manages a pool of workers that execute tasks.
type WorkerPool struct {
	tasksCh     chan workItem   // channel for incoming tasks
	ResultsCh   chan TaskResult // channel for task results
	ctx         context.Context // context for cancellation signal
	cancelFunc  context.CancelFunc
	wg          sync.WaitGroup
	stopOnError bool
	log         *log.Logger
}

// NewWorkerPool initializes the worker pool with the given number of workers.
// If stopOnError is true, the pool will cancel on the first task error.
func NewWorkerPool(parentCtx context.Context, numWorkers int, stopOnError bool, logger *log.Logger) *WorkerPool {
	// Create a cancellable context for the pool
	ctx, cancel := context.WithCancel(parentCtx)
	wp := &WorkerPool{
		tasksCh:     make(chan workItem),
		ResultsCh:   make(chan TaskResult),
		ctx:         ctx,
		cancelFunc:  cancel,
		stopOnError: stopOnError,
		log:         logger,
	}
	// Start the specified number of worker goroutines
	for i := 1; i <= numWorkers; i++ {
		go wp.worker(i)
		logger.Infof("Worker %d started", i)
	}
	// Goroutine to handle shutdown logic when context is canceled
	go func() {
		<-wp.ctx.Done() // wait until cancellation signal
		logger.Infof("Pool cancellation triggered, shutting down task dispatch")
		// Close the task channel to stop workers from picking up new tasks
		close(wp.tasksCh) // safe to close here (producer side) [oai_citation:9‡callistaenterprise.se](https://callistaenterprise.se/blogg/teknik/2019/10/05/go-worker-cancellation/#:~:text=Closing%20on%20the%20producer%20side)
		// Wait for all ongoing tasks to finish
		wp.wg.Wait()
		// All in-flight tasks are done; close the results channel and exit
		logger.Infof("All tasks completed, closing results channel")
		close(wp.ResultsCh)
	}()
	return wp
}

// Submit adds a new task to the pool. It returns an error if the pool is already canceled.
func (wp *WorkerPool) Submit(id string, taskFn TaskFunc) error {
	// First, check if the pool is shutting down (context canceled)
	select {
	case <-wp.ctx.Done():
		// Pool is canceled – reject new task
		wp.log.Warnf("Submit rejected for task %s: pool is shutting down", id)
		return errors.New("worker pool is canceled; cannot accept new tasks")
	default:
		// Pool still running, proceed to submit
	}

	// Send the task to the task channel (non-blocking select to also watch for cancel)
	select {
	case wp.tasksCh <- workItem{id: id, fn: taskFn}:
		// Task accepted into the queue
		return nil
	case <-wp.ctx.Done():
		// If cancellation happened in the middle of submission
		wp.log.Warnf("Submit failed for task %s: pool was canceled", id)
		return errors.New("worker pool is canceled; task not accepted")
	}
}

// worker is the function each worker goroutine runs to process tasks.
func (wp *WorkerPool) worker(workerID int) {
	for {
		select {
		case <-wp.ctx.Done():
			// Pool context canceled: stop accepting new work and exit
			wp.log.Infof("Worker %d exiting due to cancellation", workerID)
			return
		case task, ok := <-wp.tasksCh:
			if !ok {
				// Task channel closed (no more tasks will arrive)
				wp.log.Infof("Worker %d exiting: task channel closed", workerID)
				return
			}
			// We got a task to execute. Mark this task in progress.
			wp.wg.Add(1)
			// If cancellation was triggered just before picking up this task, log a warning.
			if wp.ctx.Err() != nil {
				wp.log.Warnf("Task %s is starting after cancellation was signaled", task.id)
			}
			wp.log.Infof("Worker %d starting task %s", workerID, task.id)
			// Execute the task function with the pool's context
			result, err := task.fn(wp.ctx)
			// Log task completion and any error
			if err != nil {
				wp.log.Errorf("Task %s failed: %v", task.id, err)
				if wp.stopOnError {
					// Cancel the pool if StopOnError is enabled and a task errored
					wp.log.Warnf("StopOnError active - canceling pool due to error in task %s", task.id)
					wp.cancelFunc() // triggers ctx.Done() for cancellation
				}
			} else {
				wp.log.Infof("Task %s completed successfully", task.id)
			}

			// Send the task result (or error) to the results channel
			wp.ResultsCh <- TaskResult{ID: task.id, Result: result, Err: err}
			wp.log.Infof("Worker %d finished task %s", workerID, task.id)
			// Mark this task as done in the WaitGroup
			wp.wg.Done()
			// Loop continues to pick up next task...
		}
	}
}

// Stop allows manually stopping the pool (cancelling its context).
// This can be used to trigger graceful shutdown if needed.
func (wp *WorkerPool) Stop() {
	wp.log.Infof("Manual stop invoked: canceling worker pool")
	wp.cancelFunc()
}
