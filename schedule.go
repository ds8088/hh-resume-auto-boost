package main

import (
	"log/slog"
	"sync"
	"time"

	"github.com/imroc/req/v3"
)

type resumeScheduler struct {
	resumes  map[string]*hhResume
	resumeMu sync.Mutex

	stopCh chan struct{}

	boostMu sync.Mutex
}

func newResumeScheduler() *resumeScheduler {
	return &resumeScheduler{
		resumes: map[string]*hhResume{},
		stopCh:  make(chan struct{}),
	}
}

func (sched *resumeScheduler) schedule(ctx *AppContext, cl *req.Client, resume *hhResume) {
	sched.resumeMu.Lock()
	defer sched.resumeMu.Unlock()

	if _, ok := sched.resumes[resume.id]; ok {
		slog.Debug("resume already scheduled, ignoring", "id", resume.id, "title", resume.title)
		return
	}

	sched.resumes[resume.id] = resume

	// Start a goroutine to handle the boost
	go sched.waitAndBoost(ctx, cl, resume)
}

func (sched *resumeScheduler) waitAndBoost(ctx *AppContext, cl *req.Client, resume *hhResume) {
	for {
		nextBoostTime := resume.lastBoost.Add(ctx.Cfg.BoostInterval)

		// If we have not yet reached the deadline, wait a bit
		now := time.Now()
		if nextBoostTime.After(now) {
			slog.Info("scheduling resume boost", "id", resume.id, "title", resume.title, "boost_time", nextBoostTime)

			timer := time.NewTimer(nextBoostTime.Sub(now))
			select {
			case <-timer.C:
			case <-sched.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}

		err := sched.exclusiveBoost(ctx, cl, resume)
		if err != nil {
			// wait a bit and retry
			slog.Info("failed to boost resume, will schedule another attempt", "error", err.Error(), "wait_for", ctx.Cfg.BoostBackoffDelay)
			timer := time.NewTimer(ctx.Cfg.BoostBackoffDelay)
			select {
			case <-timer.C:
				continue
			case <-sched.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}

		resume.lastBoost = time.Now()
	}
}

func (sched *resumeScheduler) exclusiveBoost(ctx *AppContext, cl *req.Client, resume *hhResume) error {
	sched.boostMu.Lock()
	defer sched.boostMu.Unlock()

	return hhBoostResume(ctx, cl, resume)
}

func (sched *resumeScheduler) done() <-chan struct{} {
	sched.resumeMu.Lock()
	defer sched.resumeMu.Unlock()

	// The logic is solid but the implementation is somewhat wacky.
	// If there are no resumes - return a closed channel.
	if len(sched.resumes) == 0 {
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	// If there ARE resumes, the scheduler will never evict them,
	// since it will infinitely try to re-schedule a resume
	// even if its boost fails for any reason.
	//
	// Thus, we return a non-closed channel, so that the
	// select prong for done() in main.go becomes a no-op.
	return make(chan struct{})
}

func (sched *resumeScheduler) teardown() {
	close(sched.stopCh)
}
