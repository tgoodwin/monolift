package frontend

import (
	"context"
	"time"

	"github.com/tgoodwin/monolift/demo/monolith/timelineservice"
	timelineTypes "github.com/tgoodwin/monolift/demo/monolith/types/timeline"
	"github.com/tgoodwin/monolift/demo/monolith/util"
)

// TimelineUpdateJob represents a task for our workers.
type TimelineUpdateJob struct {
	Ctx    context.Context
	Req    timelineTypes.UpdateReq
	PostId string // For logging
	UserId string // For logging
}

// TimelineUpdaterPool manages a pool of workers for updating timelines.
type TimelineUpdaterPool struct {
	jobChannel      chan TimelineUpdateJob
	timelineService timelineservice.Service
}

// NewTimelineUpdaterPool creates and starts a new worker pool.
func NewTimelineUpdaterPool(bufferSize, numWorkers int, timelineService timelineservice.Service) *TimelineUpdaterPool {
	pool := &TimelineUpdaterPool{
		jobChannel:      make(chan TimelineUpdateJob, bufferSize),
		timelineService: timelineService,
	}

	logger.Printf("Starting timeline updater pool with %d workers and a buffer of %d.", numWorkers, bufferSize)
	for i := 0; i < numWorkers; i++ {
		go pool.worker(i + 1)
	}

	return pool
}

// Submit adds a new timeline update job to the queue.
func (p *TimelineUpdaterPool) Submit(job TimelineUpdateJob) {
	// In a production system, you might want to handle the case
	// where the channel is full (e.g., log a warning, drop the job, or block).
	// For now, we assume the buffer is large enough or blocking is acceptable.
	p.jobChannel <- job
}

// worker is the main loop for a single worker goroutine.
func (p *TimelineUpdaterPool) worker(id int) {
	logger.Printf("Timeline worker %d started.", id)
	for job := range p.jobChannel {
		serviceCallStart := time.Now()
		_, err := p.timelineService.UpdateTimeline(job.Ctx, job.Req)

		util.ObserveHist(serviceCallLatHist.WithLabelValues("timelineservice", "UpdateTimeline"), float64(time.Since(serviceCallStart).Milliseconds()))

		if err != nil {
			logger.Printf("Timeline worker %d: failed to update timeline for user %s, post %s: %v", id, job.UserId, job.PostId, err)
		}
	}
}
