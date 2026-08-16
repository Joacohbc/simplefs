package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestParseConfiguration(t *testing.T) {
	tests := []struct {
		name                 string
		args                 []string
		envPort              string
		envStorageDir        string
		expectedPort         string
		expectedStorageDir   string
		expectedErrIsHelp    bool
		expectErr            bool
		expectedOutputSubstr string
	}{
		{
			name:               "default configuration when no arguments or environment variables",
			args:               []string{},
			expectedPort:       "8080",
			expectedStorageDir: "./uploads",
		},
		{
			name:               "environment variables fallback when flags are omitted",
			args:               []string{},
			envPort:            "9090",
			envStorageDir:      "/tmp/env-storage",
			expectedPort:       "9090",
			expectedStorageDir: "/tmp/env-storage",
		},
		{
			name:               "shorthand flags -p and -d",
			args:               []string{"-p", "3000", "-d", "/data/files"},
			expectedPort:       "3000",
			expectedStorageDir: "/data/files",
		},
		{
			name:               "long flags --port and --directory",
			args:               []string{"--port", "4000", "--directory", "/custom/directory"},
			expectedPort:       "4000",
			expectedStorageDir: "/custom/directory",
		},
		{
			name:               "flags override environment variables",
			args:               []string{"-p", "5000", "--directory", "/flag/directory"},
			envPort:            "8080",
			envStorageDir:      "/env/directory",
			expectedPort:       "5000",
			expectedStorageDir: "/flag/directory",
		},
		{
			name:                 "help flag with --help",
			args:                 []string{"--help"},
			expectedErrIsHelp:    true,
			expectErr:            true,
			expectedOutputSubstr: "Usage:\n  simplefs [options]",
		},
		{
			name:              "short help flag with -h",
			args:              []string{"-h"},
			expectedErrIsHelp: true,
			expectErr:         true,
		},
		{
			name:      "unknown flag returns error",
			args:      []string{"--invalid-flag"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envPort != "" {
				t.Setenv("PORT", tt.envPort)
			} else {
				t.Setenv("PORT", "")
			}

			if tt.envStorageDir != "" {
				t.Setenv("STORAGE_DIR", tt.envStorageDir)
			} else {
				t.Setenv("STORAGE_DIR", "")
			}

			var outputBuffer bytes.Buffer
			config, err := parseConfiguration(tt.args, &outputBuffer)

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if tt.expectedErrIsHelp && !errors.Is(err, flag.ErrHelp) {
					t.Errorf("expected ErrHelp, got: %v", err)
				}
				if !tt.expectedErrIsHelp && errors.Is(err, flag.ErrHelp) {
					t.Errorf("did not expect ErrHelp, got: %v", err)
				}
				if tt.expectedOutputSubstr != "" && !strings.Contains(outputBuffer.String(), tt.expectedOutputSubstr) {
					t.Errorf("expected output to contain %q, got %q", tt.expectedOutputSubstr, outputBuffer.String())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if config.Port != tt.expectedPort {
				t.Errorf("got port %q, want %q", config.Port, tt.expectedPort)
			}

			if config.StorageDirectory != tt.expectedStorageDir {
				t.Errorf("got storage directory %q, want %q", config.StorageDirectory, tt.expectedStorageDir)
			}
		})
	}
}
