package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/imroc/req/v3"
	"golang.org/x/net/html"
)

type hhResume struct {
	id        string
	title     string
	public    bool
	lastBoost time.Time

	// piggybacking the XSRF token onto the resume
	// We must use the same XSRF token when boosting the resume so this is fine
	xsrf string
}

type hhApplicantResume struct {
	Attributes struct {
		Hash                string `json:"hash"`
		HasPublicVisibility bool   `json:"hasPublicVisibility"`
		Updated             int64  `json:"updated"`
	} `json:"_attributes"`

	Title []struct {
		Data string `json:"string"`
	} `json:"title"`
}

type hhInfo struct {
	Account struct {
		Email     string `json:"email"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Phone     string `json:"phone"`
	} `json:"account"`

	ApplicantResumes []hhApplicantResume `json:"applicantResumes"`
}

// getHHInitialState retrieves a string representation of a template tag's contents;
// the template tag must have an ID "HH-Lux-InitialState".
// Basically this just does `document.querySelector('#HH-Lux-InitialState')?.innerHTML`
// except that it also attempts to merge text nodes that act as the immediate children of the template.
func getHHInitialState(doc *html.Node) string {
	for n := range doc.Descendants() {
		if n.Type == html.ElementNode && n.Data == "template" {
			for _, attr := range n.Attr {
				if attr.Key == "id" && strings.EqualFold(attr.Val, "HH-Lux-InitialState") {
					data := ""
					for node := range n.ChildNodes() {
						if node.Type == html.TextNode {
							data += node.Data
						}
					}

					return data
				}
			}
		}
	}

	return ""
}

// extractResumes transforms the raw HH info structure into an array of resumes.
func extractResumes(info *hhInfo, xsrf string) ([]hhResume, error) {
	resumes := make([]hhResume, 0, len(info.ApplicantResumes))

	for _, resume := range info.ApplicantResumes {
		titles := []string{}
		for _, t := range resume.Title {
			titles = append(titles, t.Data)
		}

		resumes = append(resumes, hhResume{
			id:        resume.Attributes.Hash,
			title:     strings.Join(titles, "; "),
			public:    resume.Attributes.HasPublicVisibility,
			lastBoost: time.UnixMilli(resume.Attributes.Updated),
			xsrf:      xsrf,
		})
	}

	return resumes, nil
}

// hhGetResumes retrieves and parses the resume list from HH.
func hhGetResumes(ctx *AppContext, cl *req.Client, noAuth bool) (iter.Seq[*hhResume], error) {
	slog.Debug("getting resume list from HH")

	r := cl.R()
	r.SetHeaders(map[string]string{
		"Sec-Fetch-Dest": "document",
		"Sec-Fetch-Mode": "navigate",
		"Sec-Fetch-Site": "same-origin",
	})

	resp, err := r.Get(buildHHURL(ctx, "/applicant/resumes?role=applicant"))
	if err != nil {
		return nil, fmt.Errorf("sending HTTP request: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	xsrf := getXSRFToken(resp)
	if xsrf == "" {
		return nil, errors.New("missing XSRF token")
	}

	if resp.StatusCode == http.StatusForbidden && !noAuth {
		slog.Debug("got HTTP/403, attempting to authenticate")

		err := hhAuthenticate(ctx, cl, xsrf)
		if err != nil {
			return nil, fmt.Errorf("authenticating in HH: %w", err)
		}

		return hhGetResumes(ctx, cl, true)
	}

	if !resp.IsSuccessState() {
		return nil, fmt.Errorf("received a HTTP error: %v", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing request body: %w", err)
	}

	slog.Debug("parsing resume list response body")

	initialState := getHHInitialState(doc)

	info := hhInfo{}
	err = json.Unmarshal([]byte(initialState), &info)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling HH initial state: %w", err)
	}

	slog.Info("extracted HH account info", "email", info.Account.Email, "name", info.Account.FirstName+" "+info.Account.LastName)
	slog.Debug("extracting resumes", "num_resumes", len(info.ApplicantResumes))

	resumes, err := extractResumes(&info, xsrf)
	if err != nil {
		return nil, fmt.Errorf("extracting resumes: %w", err)
	}

	return func(yield func(*hhResume) bool) {
		for _, resume := range resumes {
			eligible := calculateResumeEligibility(ctx, &resume)
			if !eligible {
				slog.Warn("ignoring resume due to eligibility constraints", "id", resume.id, "title", resume.title)
				continue
			}

			slog.Info("discovered resume", "id", resume.id, "title", resume.title)
			if !yield(&resume) {
				return
			}
		}
	}, nil
}

// calculateResumeEligibility checks if a resume is eligible for boosting
// according to the eligibility lists.
func calculateResumeEligibility(ctx *AppContext, resume *hhResume) bool {
	lcid := strings.ToLower(resume.id)
	lctitle := strings.ToLower(resume.title)

	// Process allowlist first
	if len(ctx.Cfg.AllowedResumes.IDs) > 0 || len(ctx.Cfg.AllowedResumes.Substrings) > 0 {
		if slices.Contains(ctx.Cfg.AllowedResumes.IDs, lcid) {
			return true
		}

		for _, substr := range ctx.Cfg.AllowedResumes.Substrings {
			if strings.Contains(lctitle, substr) {
				return true
			}
		}

		// No allowlist match
		return false
	}

	// Process private/public toggle
	if ctx.Cfg.IgnoredResumes.Public && resume.public {
		return false
	}

	if ctx.Cfg.IgnoredResumes.Private && !resume.public {
		return false
	}

	// Process blocklist
	if len(ctx.Cfg.IgnoredResumes.IDs) > 0 || len(ctx.Cfg.IgnoredResumes.Substrings) > 0 {
		if slices.Contains(ctx.Cfg.IgnoredResumes.IDs, lcid) {
			return false
		}

		for _, substr := range ctx.Cfg.IgnoredResumes.Substrings {
			if strings.Contains(lctitle, substr) {
				return false
			}
		}
	}

	// No match (or eligibility lists are not configured); allow this resume
	return true
}
