package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestWorkerPool(t *testing.T) {
	t.Run("should process all jobs", func(t *testing.T) {
		numWorkers := 2
		numJobs := 5
		doWork := func(ctx context.Context, payload interface{}) (interface{}, error) {
			jobID := payload.(int)
			return fmt.Sprintf("result for job %d", jobID), nil
		}

		pool := NewWorkerPool(numWorkers, doWork)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		pool.Start(ctx)

		for i := 0; i < numJobs; i++ {
			pool.Submit(Job{ID: i, Payload: i})
		}
		pool.Close()

		results := make(map[int]string)
		for result := range pool.Results() {
			results[result.JobID] = result.Value.(string)
		}

		if len(results) != numJobs {
			t.Errorf("expected %d results, got %d", numJobs, len(results))
		}

		for i := 0; i < numJobs; i++ {
			expected := fmt.Sprintf("result for job %d", i)
			if results[i] != expected {
				t.Errorf("expected result '%s' for job %d, got '%s'", expected, i, results[i])
			}
		}
	})

	t.Run("should handle context cancellation", func(t *testing.T) {
		numWorkers := 2
		doWork := func(ctx context.Context, payload interface{}) (interface{}, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return "done", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		pool := NewWorkerPool(numWorkers, doWork)
		ctx, cancel := context.WithCancel(context.Background())
		pool.Start(ctx)

		pool.Submit(Job{ID: 1})
		pool.Submit(Job{ID: 2})

		time.Sleep(10 * time.Millisecond) // Give workers time to pick up jobs
		cancel()

		// Drain the results channel. With the fix, this should not hang.
		for range pool.Results() {
		}
	})

	t.Run("should handle errors in doWork", func(t *testing.T) {
		numWorkers := 2
		numJobs := 5
		expectedErr := errors.New("something went wrong")
		doWork := func(ctx context.Context, payload interface{}) (interface{}, error) {
			jobID := payload.(int)
			if jobID%2 == 0 {
				return nil, expectedErr
			}
			return "success", nil
		}

		pool := NewWorkerPool(numWorkers, doWork)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		pool.Start(ctx)

		for i := 0; i < numJobs; i++ {
			pool.Submit(Job{ID: i, Payload: i})
		}
		pool.Close()

		errCount := 0
		successCount := 0
		for result := range pool.Results() {
			if result.Err != nil {
				if !errors.Is(result.Err, expectedErr) {
					t.Errorf("expected error '%v', got '%v'", expectedErr, result.Err)
				}
				errCount++
			} else {
				successCount++
			}
		}

		// Job IDs 0, 2, 4 should error.
		if errCount != 3 {
			t.Errorf("expected 3 errors, got %d", errCount)
		}
		// Job IDs 1, 3 should succeed.
		if successCount != 2 {
			t.Errorf("expected 2 successes, got %d", successCount)
		}
	})

	t.Run("should handle no jobs", func(t *testing.T) {
		numWorkers := 2
		doWork := func(ctx context.Context, payload interface{}) (interface{}, error) {
			t.Error("doWork should not be called")
			return "done", nil
		}

		pool := NewWorkerPool(numWorkers, doWork)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		pool.Start(ctx)
		pool.Close()

		count := 0
		for range pool.Results() {
			count++
		}

		if count != 0 {
			t.Errorf("expected 0 results, got %d", count)
		}
	})

	t.Run("context cancellation with blocked results channel", func(t *testing.T) {
		numWorkers := 1
		jobStarted := make(chan struct{})
		doWork := func(ctx context.Context, payload interface{}) (interface{}, error) {
			close(jobStarted)
			// This is a long running job
			time.Sleep(200 * time.Millisecond)
			return "done", nil
		}

		pool := NewWorkerPool(numWorkers, doWork)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		pool.Start(ctx)

		pool.Submit(Job{ID: 1})

		// Wait for the job to start
		<-jobStarted

		// The worker is now in doWork. It will produce a result in 200ms.
		// We are NOT reading from the results channel.

		// Cancel the context while the worker is busy and the results channel is unconsumed.
		cancel()

		// Now, we expect the pool to shut down gracefully without deadlock.
		// The test will hang here and time out if the fix is not working.
		// We can try to read from the results channel to see if it gets closed.
		select {
		case _, ok := <-pool.Results():
			if ok {
				// We might get one result if the worker sent it before noticing cancellation.
				// Read the rest to ensure the channel closes.
				for range pool.Results() {
				}
			}
			// If !ok, the channel is closed, which is what we want.
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Test timed out, worker pool likely deadlocked")
		}
	})
}
