package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/Evgen-Mutagen/go-shortener-url/internal/compress"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/configs"
	grpcServer "github.com/Evgen-Mutagen/go-shortener-url/internal/grpc"
	httpHandlers "github.com/Evgen-Mutagen/go-shortener-url/internal/http"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/logger"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/middleware"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/service"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/storage"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/urlservice"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

var (
	urlStore             *storage.Storage
	cfg                  *configs.Config
	urlService           *urlservice.URLService
	shortenerService     *service.ShortenerService
	grpcSrv              *grpcServer.ShortenerServer
	httpHandlersInstance *httpHandlers.Handlers
)

// printBuildInfo выводит информацию о версии, дате сборки и коммите
func printBuildInfo() {
	if buildVersion == "" {
		buildVersion = "N/A"
	}
	if buildDate == "" {
		buildDate = "N/A"
	}
	if buildCommit == "" {
		buildCommit = "N/A"
	}

	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)
}

func run() error {
	// Выводим информацию о сборке при старте
	printBuildInfo()

	var err error
	cfg, err = configs.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	urlStore, err = storage.NewStorage(cfg.FileStoragePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	urlService, err = urlservice.New(cfg, urlStore)
	if err != nil {
		return fmt.Errorf("failed to create URL service: %w", err)
	}

	// Создаем сервисы
	shortenerService = service.New(cfg, urlStore, urlService.Repo)
	grpcSrv = grpcServer.New(shortenerService)
	httpHandlersInstance = httpHandlers.New(shortenerService)

	loggerInstance, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer loggerInstance.Sync()

	r := chi.NewRouter()

	r.Use(middleware.AuthMiddleware)
	r.Use(compress.GzipCompress)
	r.Use(logger.WithLogging(loggerInstance))

	r.Post("/", httpHandlersInstance.ShortenURL)
	r.Post("/api/shorten", httpHandlersInstance.ShortenURLJSON)
	r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
		httpHandlersInstance.RedirectURL(w, r, chi.URLParam(r, "id"))
	})
	r.Get("/ping", httpHandlersInstance.Ping)
	r.Post("/api/shorten/batch", httpHandlersInstance.ShortenURLBatch)
	r.Get("/api/user/urls", httpHandlersInstance.GetUserURLs)
	r.Delete("/api/user/urls", httpHandlersInstance.DeleteUserURLs)

	r.Route("/api/internal", func(r chi.Router) {
		r.Use(middleware.TrustedSubnetMiddleware(cfg))
		r.Get("/stats", httpHandlersInstance.GetStats)
	})

	protocol := "HTTP"
	if cfg.EnableHTTPS {
		protocol = "HTTPS"
	}

	loggerInstance.Info("Starting server",
		zap.String("address", cfg.ServerAddress),
		zap.String("protocol", protocol),
	)
	loggerInstance.Info("Using storage file",
		zap.String("file", cfg.FileStoragePath),
	)
	if cfg.DatabaseDSN != "" {
		loggerInstance.Info("Database connection enabled")
	}
	if cfg.EnableHTTPS {
		loggerInstance.Info("HTTPS enabled")
	}

	server := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: r,
	}

	// Запуск pprof
	go func() {
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			loggerInstance.Error("Pprof server error", zap.Error(err))
		}
	}()

	// Запуск gRPC сервера
	grpcListener, err := net.Listen("tcp", cfg.GRPCServerAddress)
	if err != nil {
		loggerInstance.Info("gRPC server disabled", zap.Error(err))
	} else {
		grpcServerInstance := grpc.NewServer()
		grpcSrv.RegisterService(grpcServerInstance)

		go func() {
			loggerInstance.Info("Starting gRPC server", zap.String("address", cfg.GRPCServerAddress))
			if err := grpcServerInstance.Serve(grpcListener); err != nil {
				loggerInstance.Error("gRPC server error", zap.Error(err))
			}
		}()
	}

	// Запуск основного HTTP сервера
	serverErr := make(chan error, 1)
	go func() {
		var err error
		if cfg.EnableHTTPS {
			err = server.ListenAndServeTLS("", "")
		} else {
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Ожидание сигналов завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case <-quit:
		loggerInstance.Info("Shutting down server...")
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	// Сохраняем все несохраненные данные из файлового хранилища
	if urlStore != nil {
		if err := urlStore.Flush(); err != nil {
			loggerInstance.Error("Failed to flush storage data", zap.Error(err))
		} else {
			loggerInstance.Info("Storage data flushed successfully")
		}
	}

	if urlService.Repo != nil {
		if err := urlService.Repo.Close(); err != nil {
			return fmt.Errorf("failed to close database connection: %w", err)
		}
	}

	loggerInstance.Info("Server stopped")
	return nil
}

func main() {
	fmt.Println("Starting application...")
	if err := run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		log.Fatal(err)
	}
}
