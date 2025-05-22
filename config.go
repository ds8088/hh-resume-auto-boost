package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config represents the application configuration.
type Config struct {
	Debug     bool `json:"debug"`
	HTTPDebug bool `json:"http_debug"`

	// HeadHunter authentication parameters
	// HH does not currently support TOTP through a standalone authenticator
	// (only SMS TOTP is supported), so we elide TOTP support too,
	// at least for now
	Login    string `json:"login"`
	Password string `json:"password"`

	// HeadHunter endpoint URL
	Endpoint string `json:"endpoint"`

	// Major version of impersonated Chrome browser.
	// This should be kept up with the Chromium version history
	ChromeVersion int `json:"chrome_version"`

	// Resumes that are specified here will not be boosted.
	// This pretty much works as a blocklist
	IgnoredResumes struct {
		IDs        []string `json:"ids"`
		Substrings []string `json:"substrings"`
		Private    bool     `json:"private"` // Set to true to ignore all private resumes
		Public     bool     `json:"public"`  // Same, but for public resumes
	} `json:"ignored_resumes"`

	// If at least one resume is specified here,
	// any other resumes that are not in AllowedResumes will be ignored (allowlist)
	AllowedResumes struct {
		IDs        []string `json:"ids"`
		Substrings []string `json:"substrings"`
	} `json:"allowed_resumes"`

	// DiscoverInterval specifies how often we should update the resume list.
	// Set to 0 to disable auto-discovery.
	DiscoverInterval time.Duration `json:"discover_interval"`

	// DiscoverBackoffDelay determines how much we should wait if a discovery fails for any reason.
	DiscoverBackoffDelay time.Duration `json:"discover_backoff_delay"`

	// BoostInterval specifies the desired interval between consecutive resume boosts.
	// Should be equal to the HH builtin boost interval (currently 4 hours).
	BoostInterval time.Duration `json:"boost_interval"`

	// BoostBackoffDelay is the delay that occurs if a resume is scheduled for boosting,
	// but HH unexpectedly throws a HTTP 409 error (which means that the resume cannot be boosted yet).
	// In this case, we wait for a bit (BoostBackoffDelay) and try again.
	BoostBackoffDelay time.Duration `json:"boost_backoff_delay"`
}

// Instantiate instantiates a Config with a bunch of default values.
func (cfg *Config) Instantiate() {
	cfg.Endpoint = "https://hh.ru"
	cfg.ChromeVersion = 135

	cfg.DiscoverInterval = 150 * time.Minute
	cfg.DiscoverBackoffDelay = 5 * time.Minute

	cfg.BoostInterval = 4*time.Hour + 2*time.Minute
	cfg.BoostBackoffDelay = 90 * time.Second
}

// Load opens a JSON-formatted file specified by pathname
// and merges its contents to the Config instance.
func (cfg *Config) Load(pathname string) error {
	if pathname == "" {
		return errors.New("no pathname specified")
	}

	var f *os.File
	f, err := os.Open(filepath.Clean(pathname))
	if err != nil {
		slog.Error("failed to open config file, will be running with default settings", "error", err)
		return nil
	}

	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("failed to close config file", "error", err)
		}
	}()

	// Preserve existing debug option, if it's set
	preservedDebug := cfg.Debug

	err = json.NewDecoder(f).Decode(cfg)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	if preservedDebug {
		cfg.Debug = true
	}

	// We also have to lowercase the items in {block,allow}lists -
	// this is a low-hanging perf win
	lowercaseSlice(cfg.AllowedResumes.IDs)
	lowercaseSlice(cfg.AllowedResumes.Substrings)
	lowercaseSlice(cfg.IgnoredResumes.IDs)
	lowercaseSlice(cfg.IgnoredResumes.Substrings)

	return err
}

// Validate ensures that the Config instance's values are set correctly.
func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		return errors.New("missing HeadHunter endpoint")
	}

	endpointURL, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("parsing HeadHunter endpoint URL: %w", err)
	}

	if endpointURL.Scheme != "http" && endpointURL.Scheme != "https" {
		return fmt.Errorf("invalid HeadHunter endpoint URL scheme: \"%v\" (must be either \"http\" or \"https\")", endpointURL.Scheme)
	}

	if cfg.Login == "" {
		return errors.New("missing HeadHunter login")
	}

	if cfg.Password == "" {
		return errors.New("missing HeadHunter password")
	}

	if cfg.ChromeVersion <= 0 {
		return errors.New("invalid Chrome version")
	}

	if cfg.IgnoredResumes.Private && cfg.IgnoredResumes.Public {
		return errors.New("invalid ignore list state: both private and public resumes will be ignored")
	}

	hasAllowed := len(cfg.AllowedResumes.IDs) > 0 || len(cfg.AllowedResumes.Substrings) > 0
	hasIgnored := len(cfg.IgnoredResumes.IDs) > 0 || len(cfg.IgnoredResumes.Substrings) > 0 ||
		cfg.IgnoredResumes.Private || cfg.IgnoredResumes.Public

	if hasAllowed && hasIgnored {
		return errors.New("resume ignore list will not be enforced if some resumes are explicitly allowed")
	}

	// These bounds are mostly here to lower the load on HH infrastructure
	if cfg.BoostInterval < 10*time.Minute {
		return errors.New("resume boost interval is too low")
	}

	if cfg.BoostBackoffDelay < 30*time.Second {
		return errors.New("resume boost backoff delay is too low")
	}

	if cfg.DiscoverInterval < 10*time.Minute {
		return errors.New("resume discover interval is too low")
	}

	if cfg.DiscoverBackoffDelay < 30*time.Second {
		return errors.New("resume discover backoff delay is too low")
	}

	return nil
}

func lowercaseSlice(sl []string) {
	for i := range sl {
		sl[i] = strings.ToLower(sl[i])
	}
}
