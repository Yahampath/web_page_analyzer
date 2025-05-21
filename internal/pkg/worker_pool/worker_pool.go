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

	for i := 1; i <= numWorkers; i++ {
		go wp.worker(i)
		logger.Infof("Worker %d started", i)
	}

	go func() {
		<-wp.ctx.Done() 
		logger.Infof("Pool cancellation triggered, shutting down task dispatch")
		
		close(wp.tasksCh) 
		
	
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
	for {
		select {
		case <-wp.ctx.Done():
			wp.log.Infof("Worker %d exiting due to cancellation", workerID)
			return
		case task, ok := <-wp.tasksCh:
			if !ok {
				wp.log.Infof("Worker %d exiting: task channel closed", workerID)
				return
			}
			wp.wg.Add(1)

			if wp.ctx.Err() != nil {
				wp.log.Warnf("Task %s is starting after cancellation was signaled", task.id)
			}
			wp.log.Infof("Worker %d starting task %s", workerID, task.id)

			result, err := task.fn(wp.ctx)

			if err != nil {
				wp.log.Errorf("Task %s failed: %v", task.id, err)
				if wp.stopOnError {

					wp.log.Warnf("StopOnError active - canceling pool due to error in task %s", task.id)
					wp.cancelFunc() 
				}
			} else {
				wp.log.Infof("Task %s completed successfully", task.id)
			}

			wp.ResultsCh <- TaskResult{ID: task.id, Result: result, Err: err}
			wp.log.Infof("Worker %d finished task %s", workerID, task.id)

			wp.wg.Done()
		}
	}
}

func (wp *WorkerPool) Stop() {
	wp.log.Infof("Manual stop invoked: canceling worker pool")
	wp.cancelFunc()
}
