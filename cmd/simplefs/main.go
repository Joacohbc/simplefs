package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"simplefs/internal/handlers"
	"simplefs/internal/i18n"
	"simplefs/internal/middleware"
	"simplefs/internal/storage"
	"simplefs/web"
)

type Config struct {
	Port             string
	StorageDirectory string
}

func parseConfiguration(arguments []string, outputWriter io.Writer) (Config, error) {
	defaultPort := os.Getenv("PORT")
	if defaultPort == "" {
		defaultPort = "8080"
	}

	defaultStorageDirectory := os.Getenv("STORAGE_DIR")
	if defaultStorageDirectory == "" {
		defaultStorageDirectory = "./uploads"
	}

	flagSet := flag.NewFlagSet("simplefs", flag.ContinueOnError)
	flagSet.SetOutput(outputWriter)

	var port string
	var storageDirectory string

	flagSet.StringVar(&port, "port", defaultPort, "Port to listen on")
	flagSet.StringVar(&port, "p", defaultPort, "Port to listen on (shorthand)")
	flagSet.StringVar(&storageDirectory, "directory", defaultStorageDirectory, "Directory to serve and store files")
	flagSet.StringVar(&storageDirectory, "d", defaultStorageDirectory, "Directory to serve and store files (shorthand)")

	flagSet.Usage = func() {
		fmt.Fprintf(outputWriter, "SimpleFS - Minimalist, high-performance file server\n\n")
		fmt.Fprintf(outputWriter, "Usage:\n")
		fmt.Fprintf(outputWriter, "  simplefs [options]\n\n")
		fmt.Fprintf(outputWriter, "Options:\n")
		fmt.Fprintf(outputWriter, "  -p, --port <port>          Port to listen on (default: 8080, env: PORT)\n")
		fmt.Fprintf(outputWriter, "  -d, --directory <path>     Directory to serve and store files (default: ./uploads, env: STORAGE_DIR)\n")
		fmt.Fprintf(outputWriter, "  -h, --help                 Show this help message\n\n")
		fmt.Fprintf(outputWriter, "Environment Variables:\n")
		fmt.Fprintf(outputWriter, "  PORT                       Port to listen on (overridden by -p/--port)\n")
		fmt.Fprintf(outputWriter, "  STORAGE_DIR                Directory to serve and store files (overridden by -d/--directory)\n")
	}

	if err := flagSet.Parse(arguments); err != nil {
		return Config{}, err
	}

	return Config{
		Port:             port,
		StorageDirectory: storageDirectory,
	}, nil
}

func main() {
	configuration, err := parseConfiguration(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		os.Exit(2)
	}

	if err := os.MkdirAll(configuration.StorageDirectory, 0700); err != nil {
		log.Fatalf("Failed to initialize storage directory: %v", err)
	}

	templateEngine, err := template.New("").Funcs(template.FuncMap{
		"T": i18n.T,
	}).ParseFS(web.Assets, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	storageService := storage.NewService(configuration.StorageDirectory)
	appHandler := handlers.NewHandler(storageService, templateEngine)

	mux := http.NewServeMux()
	appHandler.RegisterRoutes(mux, web.Assets)

	absoluteStoragePath, _ := filepath.Abs(configuration.StorageDirectory)
	serverAddress := ":" + configuration.Port

	securedHandler := middleware.NewSecurityHeadersMiddleware(middleware.CSRFProtectionMiddleware(mux))

	server := &http.Server{
		Addr:              serverAddress,
		Handler:           securedHandler,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	log.Printf("🚀 SimpleFS running at http://localhost%s serving directory: %s", serverAddress, absoluteStoragePath)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server stopped: %v", err)
	}
}

