package main

import (
	"iter"
	"log/slog"
	"time"

	"github.com/imroc/req/v3"
)

// discoverResumes keeps repeatedly yielding resume instances according to the DiscoveryInterval.
// If DiscoveryInterval is zero, it performs the discovery only once and exits.
func discoverResumes(ctx *AppContext, cl *req.Client) iter.Seq[*hhResume] {
	return func(yield func(*hhResume) bool) {
		consecutiveFailures := 0

		for {
			slog.Debug("discovering resumes")

			resumes, err := hhGetResumes(ctx, cl, false)
			if err != nil {
				slog.Error("failed to get resume list", "error", err)

				// If we are not set up for rediscovery, return instantly
				if ctx.Cfg.DiscoverInterval == 0 {
					return
				}

				consecutiveFailures++
				if consecutiveFailures >= 3 {
					slog.Error("too many consecutive resume discovery failures, stopping discovery")
					return
				}

				// wait a bit and retry
				slog.Info("scheduled next discovery retry", "wait_for", ctx.Cfg.DiscoverBackoffDelay)
				timer := time.NewTimer(ctx.Cfg.DiscoverBackoffDelay)
				select {
				case <-timer.C:
					continue
				case <-ctx.Done():
					return
				}
			}

			consecutiveFailures = 0

			for resume := range resumes {
				if !yield(resume) {
					// Intentionally return here: we will have to tear down the discovery process anyway
					// if our consumer has stopped
					return
				}
			}

			if ctx.Cfg.DiscoverInterval == 0 {
				return
			}

			slog.Info("scheduled next discovery", "wait_for", ctx.Cfg.DiscoverInterval)
			timer := time.NewTimer(ctx.Cfg.DiscoverInterval)
			select {
			case <-timer.C:
			case <-ctx.Done():
				return
			}
		}
	}
}
