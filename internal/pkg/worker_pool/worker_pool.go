package worker_pool

import (
	"context"
	"errors"
	"sync"

	log "github.com/sirupsen/logrus"
)

type TaskFunc func(ctx context.Context) (any, error)

type TaskResult struct {
	ID     string
	Result any
	Err    error
}

type workItem struct {
	id string
	fn TaskFunc
}

type WorkerPool struct {
	tasksCh     chan workItem
	ResultsCh   chan TaskResult
	ctx         context.Context
	cancelFunc  context.CancelFunc
	wg          sync.WaitGroup
	stopOnError bool
	log         *log.Logger
}

func NewWorkerPool(parentCtx context.Context, numWorkers int, stopOnError bool, logger *log.Logger) *WorkerPool {
	ctx, cancel := context.WithCancel(parentCtx)
	wp := &WorkerPool{
		tasksCh:     make(chan workItem),
		ResultsCh:   make(chan TaskResult),
		ctx:         ctx,
		cancelFunc:  cancel,
		stopOnError: stopOnError,
		log:         logger,
	}

	wp.wg.Add(numWorkers)
	for i := 1; i <= numWorkers; i++ {
		go wp.worker(i)
		logger.Infof("Worker %d started", i)
	}

	go func() {
		<-wp.ctx.Done()
		logger.Infof("Pool cancellation triggered, shutting down task dispatch")
		close(wp.tasksCh)

		wp.wg.Wait()
		logger.Infof("All tasks completed, closing results channel")
		close(wp.ResultsCh)
	}()
	return wp
}

func (wp *WorkerPool) Submit(id string, taskFn TaskFunc) error {
	select {
	case <-wp.ctx.Done():
		wp.log.Warnf("Submit rejected for task %s: pool is shutting down", id)
		return errors.New("worker pool is canceled; cannot accept new tasks")
	default:
	}

	select {
	case wp.tasksCh <- workItem{id: id, fn: taskFn}:
		return nil
	case <-wp.ctx.Done():
		wp.log.Warnf("Submit failed for task %s: pool was canceled", id)
		return errors.New("worker pool is canceled; task not accepted")
	}
}

func (wp *WorkerPool) worker(workerID int) {
	defer wp.wg.Done()
	select {
	case <-wp.ctx.Done():
		wp.log.Infof("Worker %d exiting due to cancellation", workerID)
		return
	case task, ok := <-wp.tasksCh:
		if !ok {
			wp.log.Infof("Worker %d exiting: task channel closed", workerID)
			return
		}

		wp.log.Infof("Worker %d starting task %s", workerID, task.id)

		var result any
		var err error
		if task.fn != nil {
			result, err = task.fn(wp.ctx)
		} else {
			wp.log.Errorf("Task %s failed: nil task function", task.id)
			err = errors.New("nil task function")
		}

		if err != nil {
			wp.log.Errorf("Task %s failed: %v", task.id, err)
			if wp.stopOnError {
				wp.log.Warnf("StopOnError active - canceling pool due to error in task %s", task.id)
				wp.cancelFunc()
			}
		} else {
			wp.log.Infof("Task %s completed successfully", task.id)
		}

		select {
		case wp.ResultsCh <- TaskResult{ID: task.id, Result: result, Err: err}:
		case <-wp.ctx.Done():
		}

		wp.log.Infof("Worker %d finished task %s", workerID, task.id)
	}
}

func (wp *WorkerPool) Stop() {
	wp.log.Infof("Manual stop invoked: canceling worker pool")
	wp.cancelFunc()
}
