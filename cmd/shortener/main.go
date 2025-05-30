package main

import (
	"context"
	"fmt"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/compress"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/configs"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/logger"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/storage"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/urlservice"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	urlStore   *storage.Storage
	cfg        *configs.Config
	urlService *urlservice.URLService
)

func main() {
	var err error
	cfg, err = configs.LoadConfig()
	if err != nil {
		panic(err)
	}

	urlStore, err = storage.NewStorage(cfg.FileStoragePath)
	if err != nil {
		panic(fmt.Errorf("не удалось инициализировать хранилище: %v", err))
	}

	urlService = urlservice.New(cfg, urlStore)

	loggerInstance, _ := zap.NewProduction()
	defer loggerInstance.Sync()

	r := chi.NewRouter()

	r.Use(compress.GzipCompress)
	r.Use(logger.WithLogging(loggerInstance))

	r.Post("/", urlService.ShortenURL)
	r.Post("/api/shorten", urlService.ShortenURLJSON)
	r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
		urlService.RedirectURL(w, r, chi.URLParam(r, "id"))
	})

	fmt.Printf("Starting server on %s...\n", cfg.ServerAddress)
	fmt.Printf("Using storage file: %s\n", cfg.FileStoragePath)

	server := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: r,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			loggerInstance.Fatal("Server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		loggerInstance.Error("Server shutdown error", zap.Error(err))
	}

	loggerInstance.Info("Server stopped")
}
