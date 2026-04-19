package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// TestLowercaseSlice checks that lowercaseSlice correctly transforms a slice of strings.
func TestLowercaseSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"empty", nil, nil},
		{"lowercased", []string{"stuff", "more stuff"}, []string{"stuff", "more stuff"}},
		{"mixed case", []string{"Stuff", "MORE Stuff", "213A"}, []string{"stuff", "more stuff", "213a"}},
		{"numbers and symbols", []string{"Bb15(*&2F)", "t-E-s-t"}, []string{"bb15(*&2f)", "t-e-s-t"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lowercaseSlice(test.input)
			if !slices.Equal(test.input, test.expected) {
				t.Errorf("invalid lowercase result: got %v, expected %v", test.input, test.expected)
			}
		})
	}
}

// TestParseSliceString checks that parseSliceString correctly parses a string
// to a slice of strings, de-escaping as needed.
func TestParseSliceString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty string", "", nil},
		{"single value", "123", []string{"123"}},
		{"multiple values", "a,b,c", []string{"a", "b", "c"}},
		{"escaped comma", `a\,b,c`, []string{"a,b", "c"}},
		{"escaped space", `stuff\ stuff,123`, []string{"stuff stuff", "123"}},
		{"escaped backslash", `a\\b,stuff`, []string{`a\b`, "stuff"}},
		{"multiple escapes", `1\,2\,3`, []string{"1,2,3"}},
		{"wacky backslashes", `\\\\\\,\\`, []string{`\\\`, `\`}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sl, err := parseSliceString(test.input)
			if err != nil {
				t.Fatalf("parsing slice: %v", err)
			}

			if !slices.Equal(sl, test.expected) {
				t.Errorf("invalid slice result: got %v, expected %v", sl, test.expected)
			}
		})
	}

	t.Run("trailing slash", func(t *testing.T) {
		_, err := parseSliceString(`stuff\`)
		if err == nil {
			t.Fatal("expected an error but got nil")
		}
	})
}

// TestInstantiate verifies that a config struct is correctly instantiated
// with a pre-set Endpoint field.
func TestInstantiate(t *testing.T) {
	cfg := Config{}
	cfg.Instantiate()

	if cfg.Endpoint != defaultHHEndpoint {
		t.Errorf("invalid endpoint: got %q, expected %q", cfg.Endpoint, defaultHHEndpoint)
	}

	if cfg.ChromeVersion == 0 {
		t.Error("empty chrome version")
	}
}

// TestValidate checks that config is capable of validating itself.
func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name:   "empty endpoint",
			mutate: func(c *Config) { c.Endpoint = "" },
		},
		{
			name:   "invalid scheme",
			mutate: func(c *Config) { c.Endpoint = "stuff://hh.ru" },
		},
		{
			name:   "empty login",
			mutate: func(c *Config) { c.Login = "" },
		},
		{
			name:   "empty password",
			mutate: func(c *Config) { c.Password = "" },
		},
		{
			name:   "invalid chrome version",
			mutate: func(c *Config) { c.ChromeVersion = 0 },
		},
		{
			name: "all resumes ignored",
			mutate: func(c *Config) {
				c.IgnoredResumes.Private = true
				c.IgnoredResumes.Public = true
			},
		},
		{
			name: "allowed and ignored resumes conflict",
			mutate: func(c *Config) {
				c.AllowedResumes.IDs = []string{"abc"}
				c.IgnoredResumes.IDs = []string{"def"}
			},
		},
		{
			name:   "boost interval is too low",
			mutate: func(c *Config) { c.BoostInterval = time.Second },
		},
		{
			name:   "boost backoff is too low",
			mutate: func(c *Config) { c.BoostBackoffDelay = time.Second },
		},
		{
			name:   "discover backoff is too low",
			mutate: func(c *Config) { c.DiscoverBackoffDelay = time.Second },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := Config{}
			cfg.Instantiate()
			cfg.Login = "+78005553535"
			cfg.Password = "Bash1234"

			test.mutate(&cfg)
			err := cfg.Validate()

			if err == nil {
				t.Fatal("expected error in config validation but got nil")
			}
		})
	}
}

// TestLoadFromJSON checks various scenarios when loading the config from a JSON file.
func TestLoadFromJSON(t *testing.T) {
	writeTempFile := func(content string) string {
		t.Helper()

		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("writing temp file: %v", err)
		}

		return path
	}

	t.Run("empty path", func(t *testing.T) {
		cfg := Config{}
		if err := cfg.LoadFromJSON(""); err == nil {
			t.Fatal("expected error for empty path")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		path := writeTempFile("hey is this json?")
		cfg := Config{}
		if err := cfg.LoadFromJSON(path); err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("valid json sets fields", func(t *testing.T) {
		path := writeTempFile(`{
			"login": "+78005553536",
			"password": "Bash1235",
			"chrome_version": 144,
			"endpoint": "https://staging.hh.ru/"
		}`)

		cfg := Config{}
		cfg.Instantiate()
		if err := cfg.LoadFromJSON(path); err != nil {
			t.Fatalf("loading config: %v", err)
		}

		if cfg.Login != "+78005553536" {
			t.Errorf("invalid login: got %q, expected %q", cfg.Login, "+78005553536")
		}

		if cfg.ChromeVersion != 144 {
			t.Errorf("invalid chrome version: got %v, expected %v", cfg.Login, 144)
		}
	})

	t.Run("preserves debug flag", func(t *testing.T) {
		path := writeTempFile(`{"debug": false, "login": "1", "password": "2"}`)

		cfg := Config{}
		cfg.Debug = true
		if err := cfg.LoadFromJSON(path); err != nil {
			t.Fatalf("loading config: %v", err)
		}

		if !cfg.Debug {
			t.Error("debug flag should have been preserved")
		}
	})

	t.Run("lowercases blocklist entries", func(t *testing.T) {
		path := writeTempFile(`{
			"ignored_resumes": {"ids": ["TEST", "STUFF"]},
			"allowed_resumes": {"substrings": ["STUFF111"]}
		}`)

		cfg := Config{}
		if err := cfg.LoadFromJSON(path); err != nil {
			t.Fatalf("loading config: %v", err)
		}

		if cfg.IgnoredResumes.IDs[0] != "test" || cfg.IgnoredResumes.IDs[1] != "stuff" {
			t.Errorf("ignored_resumes IDs are not lowercased: %v", cfg.IgnoredResumes.IDs)
		}

		if cfg.AllowedResumes.Substrings[0] != "stuff111" {
			t.Errorf("allowed_resumes substrings are not lowercased: %v", cfg.AllowedResumes.Substrings)
		}
	})
}

// TestLoad checks various scenarios when loading the config from environment variables.
func TestLoadFromEnv(t *testing.T) {
	t.Run("config with multiple fields", func(t *testing.T) {
		cfg := Config{}
		cfg.Password = "123123"
		t.Setenv("DEBUG", "true")
		t.Setenv("LOGIN", "+78005553535")
		t.Setenv("CHROME_VERSION", "145")
		t.Setenv("BOOST_INTERVAL", "5h")
		t.Setenv("ALLOWED_RESUMES_SUBSTRINGS", `meow\ meow,1\,\,2`)
		t.Setenv("IGNORED_RESUMES_PRIVATE", "1")

		if err := cfg.LoadFromEnv(); err != nil {
			t.Fatalf("loading config from env: %v", err)
		}

		if !cfg.Debug {
			t.Error("debug field should be true")
		}

		if cfg.ChromeVersion != 145 {
			t.Errorf("invalid chrome version: got %v, expected %v", cfg.ChromeVersion, 145)
		}

		if cfg.Login != "+78005553535" {
			t.Errorf("invalid login: got %q, expected %q", cfg.Login, "+78005553535")
		}

		if cfg.BoostInterval != 5*time.Hour {
			t.Errorf("invalid BoostInterval: got %v, want 5h", cfg.BoostInterval)
		}

		if !slices.Equal(cfg.AllowedResumes.Substrings, []string{"meow meow", "1,,2"}) {
			t.Errorf("invalid AllowedResumes.Substrings: %v", cfg.AllowedResumes.Substrings)
		}

		if !cfg.IgnoredResumes.Private {
			t.Error("IgnoredResumes.Private should be true")
		}

		if cfg.Password != "123123" {
			t.Error("password should have been preserved")
		}
	})

	t.Run("invalid int returns error", func(t *testing.T) {
		cfg := Config{}
		t.Setenv("CHROME_VERSION", "abc")

		if err := cfg.LoadFromEnv(); err == nil {
			t.Fatal("expected error for invalid int")
		}
	})

	t.Run("invalid slice returns error", func(t *testing.T) {
		cfg := Config{}
		t.Setenv("IGNORED_RESUMES_IDS", `abc\`)

		if err := cfg.LoadFromEnv(); err == nil {
			t.Fatal("expected error for non-terminated escape")
		}
	})
}
