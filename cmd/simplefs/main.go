package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"simplefs/internal/assets"
	"simplefs/internal/auth"
	"simplefs/internal/handlers"
	"simplefs/internal/middleware"
	"simplefs/internal/storage"
	"simplefs/web"
)

var version = "dev"

type Config struct {
	Port             string
	Bind             string
	StorageDirectory string
	Token            string
	ReadToken        string
	NoAuth           bool
	AllowedHosts     []string
	MaxUploadBytes   int64
	FileMode         os.FileMode
	DirMode          os.FileMode
	HideDotfiles     bool
	AccessLog        bool
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return value
}

// secretFromEnv reads KEY or, for Docker secrets, the file named by KEY_FILE.
func secretFromEnv(key string) (string, error) {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value, nil
	}
	if path := strings.TrimSpace(os.Getenv(key + "_FILE")); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("reading %s_FILE: %w", key, err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return "", nil
}

// ParseSize accepts plain bytes or a K/M/G/T suffix (binary units). Zero means unlimited.
func ParseSize(value string) (int64, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	trimmed = strings.TrimSuffix(strings.TrimSuffix(trimmed, "IB"), "B")
	if trimmed == "" {
		return 0, nil
	}
	multiplier := int64(1)
	switch trimmed[len(trimmed)-1] {
	case 'K':
		multiplier = 1 << 10
	case 'M':
		multiplier = 1 << 20
	case 'G':
		multiplier = 1 << 30
	case 'T':
		multiplier = 1 << 40
	}
	if multiplier > 1 {
		trimmed = trimmed[:len(trimmed)-1]
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(trimmed), 64)
	if err != nil || number < 0 {
		return 0, fmt.Errorf("invalid size %q", value)
	}
	return int64(number * float64(multiplier)), nil
}

func parseMode(value string) (os.FileMode, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 8, 32)
	if err != nil || parsed > 0777 {
		return 0, fmt.Errorf("invalid permission mode %q (use octal like 0644)", value)
	}
	return os.FileMode(parsed), nil
}

func splitList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func parseConfiguration(arguments []string, outputWriter io.Writer) (Config, error) {
	token, err := secretFromEnv("SIMPLEFS_TOKEN")
	if err != nil {
		return Config{}, err
	}
	readToken, err := secretFromEnv("SIMPLEFS_READ_TOKEN")
	if err != nil {
		return Config{}, err
	}

	flagSet := flag.NewFlagSet("simplefs", flag.ContinueOnError)
	flagSet.SetOutput(outputWriter)

	var config Config
	var allowedHosts, maxUpload, fileMode, dirMode string

	defaultPort := envOr("PORT", "8080")
	defaultBind := envOr("BIND", "")
	defaultDirectory := envOr("STORAGE_DIR", "./uploads")

	flagSet.StringVar(&config.Port, "port", defaultPort, "")
	flagSet.StringVar(&config.Port, "p", defaultPort, "")
	flagSet.StringVar(&config.Bind, "bind", defaultBind, "")
	flagSet.StringVar(&config.Bind, "b", defaultBind, "")
	flagSet.StringVar(&config.StorageDirectory, "directory", defaultDirectory, "")
	flagSet.StringVar(&config.StorageDirectory, "d", defaultDirectory, "")
	flagSet.StringVar(&config.Token, "token", token, "")
	flagSet.StringVar(&config.ReadToken, "read-token", readToken, "")
	flagSet.BoolVar(&config.NoAuth, "no-auth", envBool("SIMPLEFS_NO_AUTH", false), "")
	flagSet.StringVar(&allowedHosts, "allowed-hosts", envOr("ALLOWED_HOSTS", ""), "")
	flagSet.StringVar(&maxUpload, "max-upload", envOr("MAX_UPLOAD_SIZE", "0"), "")
	flagSet.StringVar(&fileMode, "file-mode", envOr("FILE_MODE", "0644"), "")
	flagSet.StringVar(&dirMode, "dir-mode", envOr("DIR_MODE", "0755"), "")
	flagSet.BoolVar(&config.HideDotfiles, "hide-dotfiles", envBool("HIDE_DOTFILES", false), "")
	flagSet.BoolVar(&config.AccessLog, "access-log", envBool("ACCESS_LOG", true), "")

	flagSet.Usage = func() { printUsage(outputWriter) }

	if err := flagSet.Parse(arguments); err != nil {
		return Config{}, err
	}

	config.AllowedHosts = splitList(allowedHosts)
	if config.MaxUploadBytes, err = ParseSize(maxUpload); err != nil {
		return Config{}, err
	}
	if config.FileMode, err = parseMode(fileMode); err != nil {
		return Config{}, err
	}
	if config.DirMode, err = parseMode(dirMode); err != nil {
		return Config{}, err
	}
	if config.ReadToken != "" && config.ReadToken == config.Token {
		return Config{}, errors.New("--read-token must differ from --token")
	}
	return config, nil
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `SimpleFS - lightweight file server for fast file exchange

Usage:
  simplefs [options]            start the server
  simplefs healthcheck          exit 0 when the local server answers /healthz
  simplefs token                print the access token in use
  simplefs version              print the version

Options:
  -p, --port <port>          Port to listen on (default 8080, env PORT)
  -b, --bind <address>       Address to bind, e.g. 127.0.0.1 or a VPN IP (default all interfaces, env BIND)
  -d, --directory <path>     Directory to serve and store files (default ./uploads, env STORAGE_DIR)
      --token <token>        Full-access token (env SIMPLEFS_TOKEN or SIMPLEFS_TOKEN_FILE).
                             When unset a random token is generated and stored in <directory>/.simplefs/token
      --read-token <token>   Optional read-only token (env SIMPLEFS_READ_TOKEN or SIMPLEFS_READ_TOKEN_FILE)
      --no-auth              Disable authentication (env SIMPLEFS_NO_AUTH=true). Only behind another auth layer
      --allowed-hosts <list> Extra host names accepted in the Host header, comma separated; IPs and localhost
                             are always accepted, ".example.com" matches subdomains, "*" disables the check
                             (env ALLOWED_HOSTS)
      --max-upload <size>    Maximum upload size such as 512M or 20G; 0 means unlimited (env MAX_UPLOAD_SIZE)
      --file-mode <mode>     Permissions for new files (default 0644, env FILE_MODE)
      --dir-mode <mode>      Permissions for new folders (default 0755, env DIR_MODE)
      --hide-dotfiles        Hide and protect names starting with "." (env HIDE_DOTFILES=true)
      --access-log           Log one line per request (default true, env ACCESS_LOG)
  -h, --help                 Show this help message
`)
}

func tokenFilePath(storageDirectory string) string {
	return filepath.Join(storageDirectory, ".simplefs", "token")
}

func buildAuthenticator(config Config) (*auth.Authenticator, error) {
	if config.NoAuth {
		return auth.Disabled(), nil
	}
	token := config.Token
	if token == "" {
		path := tokenFilePath(config.StorageDirectory)
		stored, created, err := auth.LoadOrCreateToken(path)
		if err != nil {
			return nil, fmt.Errorf("cannot create access token: %w", err)
		}
		token = stored
		if created {
			log.Printf("🔑 Generated access token (stored in %s): %s", path, token)
		} else {
			log.Printf("🔑 Using access token stored in %s (show it with: simplefs token)", path)
		}
	}
	return auth.New(token, config.ReadToken), nil
}

func listenAddress(config Config) string {
	return net.JoinHostPort(config.Bind, config.Port)
}

func runHealthcheck(config Config) int {
	host := config.Bind
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + net.JoinHostPort(host, config.Port) + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, "unhealthy:", err)
		return 1
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "unhealthy: status", response.StatusCode)
		return 1
	}
	return 0
}

func runPrintToken(config Config) int {
	if config.Token != "" {
		fmt.Println(config.Token)
		return 0
	}
	data, err := os.ReadFile(tokenFilePath(config.StorageDirectory))
	if err != nil {
		fmt.Fprintln(os.Stderr, "no token found; start the server once or set SIMPLEFS_TOKEN")
		return 1
	}
	fmt.Println(strings.TrimSpace(string(data)))
	return 0
}

func buildServer(config Config, authenticator *auth.Authenticator) (*http.Server, error) {
	staticAssets, err := assets.New(web.Assets, "static")
	if err != nil {
		return nil, fmt.Errorf("failed to load static assets: %w", err)
	}

	templateEngine, err := handlers.ParseTemplates(web.Assets, staticAssets)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	storageService := storage.NewServiceWithOptions(config.StorageDirectory, storage.Options{
		FileMode:     config.FileMode,
		DirMode:      config.DirMode,
		HideDotfiles: config.HideDotfiles,
	})
	if err := storageService.CleanupTemp(); err != nil {
		log.Printf("warning: cannot clean temporary uploads: %v", err)
	}

	appHandler := handlers.NewHandler(storageService, templateEngine, handlers.Config{
		Auth:           authenticator,
		Assets:         staticAssets,
		MaxUploadBytes: config.MaxUploadBytes,
	})
	mux := http.NewServeMux()
	appHandler.RegisterRoutes(mux)

	var handler http.Handler = middleware.RecordRole(mux)
	handler = authenticator.Middleware(handler)
	handler = middleware.CSRFProtectionMiddleware(handler)
	handler = middleware.NewHostGuard(config.AllowedHosts).Middleware(handler)
	handler = middleware.NewSecurityHeadersMiddleware(handler)
	if config.AccessLog {
		handler = middleware.AccessLog(log.Default(), handler)
	}

	// No global read/write timeouts: transfers of any size must be able to finish.
	// Slow-header clients are still cut by ReadHeaderTimeout and UI routes set their own deadlines.
	return &http.Server{
		Addr:              listenAddress(config),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}, nil
}

func main() {
	arguments := os.Args[1:]
	command := ""
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		command, arguments = arguments[0], arguments[1:]
	}

	config, err := parseConfiguration(arguments, os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}

	switch command {
	case "":
	case "healthcheck":
		os.Exit(runHealthcheck(config))
	case "token":
		os.Exit(runPrintToken(config))
	case "version":
		fmt.Println(version)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", command)
		printUsage(os.Stderr)
		os.Exit(2)
	}

	if err := os.MkdirAll(config.StorageDirectory, config.DirMode); err != nil {
		log.Fatalf("Failed to initialize storage directory: %v", err)
	}

	authenticator, err := buildAuthenticator(config)
	if err != nil {
		log.Fatal(err)
	}

	server, err := buildServer(config, authenticator)
	if err != nil {
		log.Fatal(err)
	}

	absoluteStoragePath, _ := filepath.Abs(config.StorageDirectory)
	displayHost := config.Bind
	if displayHost == "" {
		displayHost = "localhost"
	}
	log.Printf("🚀 SimpleFS %s listening on %s (http://%s) serving %s", version, server.Addr, net.JoinHostPort(displayHost, config.Port), absoluteStoragePath)
	if !authenticator.Enabled() {
		log.Printf("⚠️  Authentication is DISABLED: anyone who can reach %s has full access", server.Addr)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server stopped: %v", err)
		}
	case <-ctx.Done():
		log.Printf("Shutting down, waiting up to 30s for active transfers...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Forced shutdown: %v", err)
		}
	}
}
