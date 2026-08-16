package main

import (
	"html/template"
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

func main() {
	storageDirectory := os.Getenv("STORAGE_DIR")
	if storageDirectory == "" {
		storageDirectory = "./uploads"
	}

	if err := os.MkdirAll(storageDirectory, 0700); err != nil {
		log.Fatalf("Failed to initialize storage directory: %v", err)
	}

	templateEngine, err := template.New("").Funcs(template.FuncMap{
		"T": i18n.T,
	}).ParseFS(web.Assets, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	storageService := storage.NewService(storageDirectory)
	appHandler := handlers.NewHandler(storageService, templateEngine)

	mux := http.NewServeMux()
	appHandler.RegisterRoutes(mux, web.Assets)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	absoluteStoragePath, _ := filepath.Abs(storageDirectory)
	serverAddress := ":" + port

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
