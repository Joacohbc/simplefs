package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
			clearEnv(t)
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

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"PORT", "BIND", "STORAGE_DIR", "SIMPLEFS_TOKEN", "SIMPLEFS_TOKEN_FILE", "SIMPLEFS_READ_TOKEN", "SIMPLEFS_READ_TOKEN_FILE",
		"SIMPLEFS_NO_AUTH", "ALLOWED_HOSTS", "MAX_UPLOAD_SIZE", "FILE_MODE", "DIR_MODE", "HIDE_DOTFILES", "ACCESS_LOG"} {
		t.Setenv(key, "")
	}
}

func TestParseConfiguration_SecurityOptions(t *testing.T) {
	clearEnv(t)
	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SIMPLEFS_TOKEN_FILE", tokenFile)
	t.Setenv("ALLOWED_HOSTS", "files.example.com, .ts.net")
	t.Setenv("MAX_UPLOAD_SIZE", "2G")
	t.Setenv("BIND", "100.64.0.1")

	config, err := parseConfiguration([]string{"--file-mode", "0640", "--read-token", "reader"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if config.Token != "from-file" || config.ReadToken != "reader" {
		t.Errorf("unexpected tokens: %q %q", config.Token, config.ReadToken)
	}
	if len(config.AllowedHosts) != 2 || config.AllowedHosts[1] != ".ts.net" {
		t.Errorf("unexpected allowed hosts %v", config.AllowedHosts)
	}
	if config.MaxUploadBytes != 2<<30 || config.FileMode != 0640 || config.DirMode != 0755 {
		t.Errorf("unexpected limits: %d %v %v", config.MaxUploadBytes, config.FileMode, config.DirMode)
	}
	if listenAddress(config) != "100.64.0.1:8080" {
		t.Errorf("unexpected listen address %q", listenAddress(config))
	}

	if _, err := parseConfiguration([]string{"--token", "same", "--read-token", "same"}, io.Discard); err == nil {
		t.Error("identical read and write tokens must be rejected")
	}
	if _, err := parseConfiguration([]string{"--file-mode", "999"}, io.Discard); err == nil {
		t.Error("invalid file mode must be rejected")
	}
	if _, err := parseConfiguration([]string{"--max-upload", "lots"}, io.Discard); err == nil {
		t.Error("invalid size must be rejected")
	}
}

func TestParseSize(t *testing.T) {
	cases := map[string]int64{"": 0, "0": 0, "1024": 1024, "512K": 512 << 10, "10MB": 10 << 20, "1.5G": 3 << 29, "2GiB": 2 << 30, "1t": 1 << 40}
	for input, expected := range cases {
		got, err := ParseSize(input)
		if err != nil || got != expected {
			t.Errorf("ParseSize(%q) = %d, %v; want %d", input, got, err, expected)
		}
	}
}

func TestServerIntegration(t *testing.T) {
	clearEnv(t)
	storageDir := t.TempDir()
	config, err := parseConfiguration([]string{"-d", storageDir, "--access-log=false"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	authenticator, err := buildAuthenticator(config)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(tokenFilePath(storageDir))
	if err != nil {
		t.Fatalf("a token should be generated and stored: %v", err)
	}
	token := strings.TrimSpace(string(stored))

	server, err := buildServer(config, authenticator)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.Handler)
	defer httpServer.Close()

	request := func(method, path, host string, headers map[string]string, body io.Reader) *http.Response {
		req, _ := http.NewRequest(method, httpServer.URL+path, body)
		if host != "" {
			req.Host = host
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	bearer := map[string]string{"Authorization": "Bearer " + token}

	if resp := request(http.MethodGet, "/healthz", "", nil, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("healthz: %d", resp.StatusCode)
	}
	if resp := request(http.MethodGet, "/api/v1/whoami", "", nil, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated API should be 401, got %d", resp.StatusCode)
	}
	resp := request(http.MethodGet, "/", "", bearer, nil)
	if resp.StatusCode != http.StatusOK || strings.Contains(resp.Header.Get("Content-Security-Policy"), "unsafe-inline") {
		t.Errorf("index: %d csp=%q", resp.StatusCode, resp.Header.Get("Content-Security-Policy"))
	}
	if resp := request(http.MethodGet, "/api/v1/whoami", "attacker.example", bearer, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("DNS rebinding host should be rejected, got %d", resp.StatusCode)
	}
	crossSite := map[string]string{"Authorization": "Bearer " + token, "Origin": "http://evil.example"}
	if resp := request(http.MethodPut, "/api/v1/files/x.txt", "", crossSite, strings.NewReader("x")); resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-origin write should be rejected, got %d", resp.StatusCode)
	}
	if resp := request(http.MethodPut, "/api/v1/files/inbox/x.txt", "", bearer, strings.NewReader("x")); resp.StatusCode != http.StatusCreated {
		t.Errorf("authenticated PUT should succeed, got %d", resp.StatusCode)
	}
	if resp := request(http.MethodGet, "/static/style.css", "", map[string]string{"Accept-Encoding": "gzip"}, nil); resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Errorf("static asset: %d", resp.StatusCode)
	}
}
