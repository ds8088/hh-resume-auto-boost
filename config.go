package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const defaultHHEndpoint = "https://hh.ru"

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
	// but HH unexpectedly throws an HTTP 409 error (which means that the resume cannot be boosted yet).
	// In this case, we wait for a bit (BoostBackoffDelay) and try again.
	BoostBackoffDelay time.Duration `json:"boost_backoff_delay"`

	// CookieJarFileName is the name of a file which will be used to store persistent cookies.
	// If empty, cookie persistence is disabled.
	CookieJarFileName string `json:"cookie_jar_file_name"`
}

// Instantiate instantiates a Config with a bunch of default values.
func (cfg *Config) Instantiate() {
	cfg.Endpoint = defaultHHEndpoint
	cfg.ChromeVersion = 147

	cfg.DiscoverInterval = 150 * time.Minute
	cfg.DiscoverBackoffDelay = 5 * time.Minute

	cfg.BoostInterval = 4*time.Hour + 2*time.Minute
	cfg.BoostBackoffDelay = 90 * time.Second

	cfg.CookieJarFileName = "cookies.json"
}

// LoadFromJSON opens a JSON-formatted file specified by pathname
// and merges its contents to the Config instance.
func (cfg *Config) LoadFromJSON(pathname string) error {
	if pathname == "" {
		return errors.New("no pathname specified")
	}

	var f *os.File
	f, err := os.Open(filepath.Clean(pathname))
	if err != nil {
		slog.Warn("failed to open config file", "error", err)
		return nil
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			slog.Error("failed to close config file", "error", closeErr)
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

	return nil
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

	if cfg.DiscoverBackoffDelay < 30*time.Second {
		return errors.New("resume discover backoff delay is too low")
	}

	return nil
}

// LoadFromEnv sets the Config instance according to the environment variables.
//
// Environment variable names are derived from json struct tags
// (uppercased, of course).
// Names of nested structs are concatenated with the underscore symbol.
// Slices are parsed as a list of comma-separated values.
//
// If an environment variable is empty or missing, it is skipped.
func (cfg *Config) LoadFromEnv() error {
	return loadEnvFromStruct(reflect.ValueOf(cfg).Elem(), "")
}

// loadEnvFromStruct iterates over a reflect-wrapped struct,
// setting config values if it finds a non-empty environment variable.
func loadEnvFromStruct(v reflect.Value, prefix string) error {
	t := v.Type()

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() && !f.Anonymous {
			continue // Ignore non-exported fields.
		}

		tag, ok := f.Tag.Lookup("json")
		if !ok || tag == "" || tag == "-" {
			continue // Not a valid tag, or the tag is empty.
		}

		// Construct the env var name.
		tag, _, _ = strings.Cut(tag, ",")
		envName := strings.ToUpper(prefix + tag)

		val := v.Field(i)

		// Iterate over nested structs.
		if f.Type.Kind() == reflect.Struct {
			err := loadEnvFromStruct(val, envName+"_")
			if err != nil {
				return err
			}

			continue
		}

		envVal := os.Getenv(envName)
		if envVal == "" {
			continue
		}

		// Type-switch over the field's kind.
		switch f.Type.Kind() {
		case reflect.String:
			val.SetString(envVal)

		case reflect.Bool:
			v, err := strconv.ParseBool(envVal)
			if err != nil {
				return fmt.Errorf("parsing boolean env var %q: %w", envName, err)
			}

			val.SetBool(v)

		case reflect.Int:
			v, err := strconv.Atoi(envVal)
			if err != nil {
				return fmt.Errorf("parsing integer env var %q: %w", envName, err)
			}

			val.SetInt(int64(v))

		case reflect.Int64:
			if f.Type == reflect.TypeFor[time.Duration]() {
				v, err := time.ParseDuration(envVal)
				if err != nil {
					return fmt.Errorf("parsing duration env var %q: %w", envName, err)
				}

				val.SetInt(int64(v))
			} else {
				v, err := strconv.ParseInt(envVal, 10, 64)
				if err != nil {
					return fmt.Errorf("parsing integer env var %q: %w", envName, err)
				}

				val.SetInt(v)
			}

		case reflect.Slice:
			v, err := parseSliceString(envVal)
			if err != nil {
				return fmt.Errorf("parsing slice env var %q: %w", envName, err)
			}

			val.Set(reflect.ValueOf(v))
		}
	}

	return nil
}

// parseSliceString splits a string by commas, supporting backslash escaping.
func parseSliceString(s string) (res []string, err error) {
	cur := strings.Builder{}
	escaped := false

	for _, c := range s {
		if escaped {
			cur.WriteRune(c)
			escaped = false
			continue
		}

		if c == '\\' {
			escaped = true
			continue
		}

		if c == ',' {
			res = append(res, cur.String())
			cur.Reset()
			continue
		}

		cur.WriteRune(c)
	}

	if escaped {
		return nil, errors.New("non-terminated escape sequence")
	}

	if cur.Len() > 0 {
		res = append(res, cur.String())
	}

	return res, nil
}

func lowercaseSlice(sl []string) {
	for i := range sl {
		sl[i] = strings.ToLower(sl[i])
	}
}
