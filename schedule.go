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

	doneCh chan struct{}
	stopCh chan struct{}

	boostMu sync.Mutex
}

func newResumeScheduler() *resumeScheduler {
	return &resumeScheduler{
		resumes: map[string]*hhResume{},
		doneCh:  make(chan struct{}),
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

		// If we're not yet reached the deadline, wait a bit
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

	if len(sched.resumes) == 0 {
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	return sched.doneCh
}

func (sched *resumeScheduler) teardown() {
	close(sched.stopCh)
}
