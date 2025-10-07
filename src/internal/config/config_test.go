package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		setup   func()
		cleanup func()
		wantErr bool
	}{
		{
			name: "all required env vars set",
			setup: func() {
				os.Setenv("NEWSNAB_API_KEY", "test_api_key")
				os.Setenv("NEWSNAB_HOST", "test_host")
				os.Setenv("DOWNLOAD_DIR", "/tmp/test_download")
				os.Setenv("TRAKT_API_KEY", "test_trakt_key")
				os.Setenv("TRAKT_CLIENT_SECRET", "test_secret")
				os.Setenv("NZBGET_URL", "http://localhost")
				os.Setenv("NZBGET_USER", "test_user")
				os.Setenv("NZBGET_PASS", "test_pass")
			},
			cleanup: func() {
				os.Unsetenv("NEWSNAB_API_KEY")
				os.Unsetenv("NEWSNAB_HOST")
				os.Unsetenv("DOWNLOAD_DIR")
				os.Unsetenv("TRAKT_API_KEY")
				os.Unsetenv("TRAKT_CLIENT_SECRET")
				os.Unsetenv("NZBGET_URL")
				os.Unsetenv("NZBGET_USER")
				os.Unsetenv("NZBGET_PASS")
				os.RemoveAll("/tmp/test_download")
			},
			wantErr: false,
		},
		{
			name: "missing required env var",
			setup: func() {
				os.Unsetenv("NEWSNAB_API_KEY")
			},
			cleanup: func() {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer tt.cleanup()

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if cfg.TaskInterval != defaultTaskInterval {
					t.Errorf("TaskInterval = %v, want %v", cfg.TaskInterval, defaultTaskInterval)
				}
				if cfg.HTTPTimeout != defaultHTTPTimeout {
					t.Errorf("HTTPTimeout = %v, want %v", cfg.HTTPTimeout, defaultHTTPTimeout)
				}
			}
		})
	}
}

func TestConfig_Paths(t *testing.T) {
	cfg := &Config{
		DataDir: "/test/data",
	}

	tests := []struct {
		name   string
		method func() string
		want   string
	}{
		{
			name:   "DBPath",
			method: cfg.DBPath,
			want:   "/test/data/data.db",
		},
		{
			name:   "TokenPath",
			method: cfg.TokenPath,
			want:   "/test/data/token.json",
		},
		{
			name:   "BlacklistPath",
			method: cfg.BlacklistPath,
			want:   "/test/data/blacklist.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.method()
			if got != tt.want {
				t.Errorf("%s() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
	}{
		{
			name:         "env var set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "custom",
			want:         "custom",
		},
		{
			name:         "env var not set",
			key:          "TEST_VAR_MISSING",
			defaultValue: "default",
			envValue:     "",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnvOrDefault(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvOrDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}
