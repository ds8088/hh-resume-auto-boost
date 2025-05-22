package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/imroc/req/v3"
)

var ErrBoostTooEarly = errors.New("resume cannot be boosted yet (too early)")

// hhBoostResume boots a single resume using the HeadHunter API.
func hhBoostResume(ctx *AppContext, cl *req.Client, resume *hhResume) error {
	slog.Debug("boosting resume", "title", resume.title)

	r := cl.R()
	r.SetHeaders(map[string]string{
		"Sec-Fetch-Dest":   "empty",
		"Sec-Fetch-Mode":   "cors",
		"Sec-Fetch-Site":   "same-origin",
		"X-Requested-With": "XMLHTTPRequest",
		"X-Xsrftoken":      resume.xsrf,
		"Accept":           "application/json",
		"Referer":          buildHHURL(ctx, "/applicant/resumes?role=applicant"),
	})

	r.EnableForceMultipart()

	r.SetFormData(map[string]string{
		"resume":       resume.id,
		"undirectable": "true",
	})

	setGSSHeaders(cl, r)

	resp, err := r.Post(buildHHURL(ctx, "/applicant/resumes/touch"))
	if err != nil {
		return fmt.Errorf("sending HTTP request: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode == http.StatusConflict {
		return ErrBoostTooEarly
	}

	if !resp.IsSuccessState() {
		return fmt.Errorf("received a HTTP error: %v", resp.StatusCode)
	}

	slog.Info("boosted resume", "title", resume.title)
	return nil
}
