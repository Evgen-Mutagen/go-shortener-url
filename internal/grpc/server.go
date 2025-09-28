package grpc

import (
	"context"

	"github.com/Evgen-Mutagen/go-shortener-url/internal/service"
	"google.golang.org/grpc"
)

// ShortenerServer gRPC сервер для сервиса сокращения URL
type ShortenerServer struct {
	service *service.ShortenerService
}

// New создает новый gRPC сервер
func New(service *service.ShortenerService) *ShortenerServer {
	return &ShortenerServer{
		service: service,
	}
}

// RegisterService регистрирует сервис в gRPC сервере
func (s *ShortenerServer) RegisterService(server *grpc.Server) {
	// Пока что просто заглушка - в реальном проекте здесь была бы регистрация
	// server.RegisterService(&shortenerpb.ShortenerService_ServiceDesc, s)
}

// ShortenURLRequest запрос на сокращение URL
type ShortenURLRequest struct {
	Url    string
	UserId string
}

// ShortenURLResponse ответ на сокращение URL
type ShortenURLResponse struct {
	ShortUrl   string
	Error      string
	StatusCode int32
}

// GetURLRequest запрос на получение оригинального URL
type GetURLRequest struct {
	Id string
}

// GetURLResponse ответ с оригинальным URL
type GetURLResponse struct {
	OriginalUrl string
	IsDeleted   bool
	Error       string
	StatusCode  int32
}

// BatchItem элемент пакетного запроса
type BatchItem struct {
	CorrelationId string
	OriginalUrl   string
}

// ShortenURLBatchRequest запрос на пакетное сокращение URL
type ShortenURLBatchRequest struct {
	Items  []BatchItem
	UserId string
}

// BatchResponseItem элемент ответа на пакетный запрос
type BatchResponseItem struct {
	CorrelationId string
	ShortUrl      string
}

// ShortenURLBatchResponse ответ на пакетное сокращение URL
type ShortenURLBatchResponse struct {
	Items      []BatchResponseItem
	Error      string
	StatusCode int32
}

// GetUserURLsRequest запрос на получение URL пользователя
type GetUserURLsRequest struct {
	UserId string
}

// UserURL элемент URL пользователя
type UserURL struct {
	ShortUrl    string
	OriginalUrl string
}

// GetUserURLsResponse ответ с URL пользователя
type GetUserURLsResponse struct {
	Urls       []UserURL
	Error      string
	StatusCode int32
}

// DeleteUserURLsRequest запрос на удаление URL пользователя
type DeleteUserURLsRequest struct {
	UrlIds []string
	UserId string
}

// DeleteUserURLsResponse ответ на удаление URL
type DeleteUserURLsResponse struct {
	Error      string
	StatusCode int32
}

// PingRequest запрос проверки доступности
type PingRequest struct{}

// PingResponse ответ на проверку доступности
type PingResponse struct {
	Error      string
	StatusCode int32
}

// GetStatsRequest запрос статистики
type GetStatsRequest struct{}

// GetStatsResponse ответ со статистикой
type GetStatsResponse struct {
	Urls       int32
	Users      int32
	Error      string
	StatusCode int32
}

// ShortenURL сокращает URL
func (s *ShortenerServer) ShortenURL(ctx context.Context, req *ShortenURLRequest) (*ShortenURLResponse, error) {
	result := s.service.ShortenURL(ctx, req.Url, req.UserId)
	return &ShortenURLResponse{
		ShortUrl:   result.ShortURL,
		Error:      result.Error,
		StatusCode: int32(result.Status),
	}, nil
}

// GetURL получает оригинальный URL по ID
func (s *ShortenerServer) GetURL(ctx context.Context, req *GetURLRequest) (*GetURLResponse, error) {
	result := s.service.GetURL(ctx, req.Id)
	return &GetURLResponse{
		OriginalUrl: result.OriginalURL,
		IsDeleted:   result.IsDeleted,
		Error:       result.Error,
		StatusCode:  int32(result.Status),
	}, nil
}

// ShortenURLBatch пакетное сокращение URL
func (s *ShortenerServer) ShortenURLBatch(ctx context.Context, req *ShortenURLBatchRequest) (*ShortenURLBatchResponse, error) {
	items := make([]service.BatchItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = service.BatchItem{
			CorrelationID: item.CorrelationId,
			OriginalURL:   item.OriginalUrl,
		}
	}

	result := s.service.ShortenURLBatch(ctx, items, req.UserId)

	responseItems := make([]BatchResponseItem, len(result.Items))
	for i, item := range result.Items {
		responseItems[i] = BatchResponseItem{
			CorrelationId: item.CorrelationID,
			ShortUrl:      item.ShortURL,
		}
	}

	return &ShortenURLBatchResponse{
		Items:      responseItems,
		Error:      result.Error,
		StatusCode: int32(result.Status),
	}, nil
}

// GetUserURLs получает все URL пользователя
func (s *ShortenerServer) GetUserURLs(ctx context.Context, req *GetUserURLsRequest) (*GetUserURLsResponse, error) {
	result := s.service.GetUserURLs(ctx, req.UserId)

	urls := make([]UserURL, len(result.URLs))
	for i, url := range result.URLs {
		urls[i] = UserURL{
			ShortUrl:    url.ShortURL,
			OriginalUrl: url.OriginalURL,
		}
	}

	return &GetUserURLsResponse{
		Urls:       urls,
		Error:      result.Error,
		StatusCode: int32(result.Status),
	}, nil
}

// DeleteUserURLs помечает URL как удаленные
func (s *ShortenerServer) DeleteUserURLs(ctx context.Context, req *DeleteUserURLsRequest) (*DeleteUserURLsResponse, error) {
	result := s.service.DeleteUserURLs(ctx, req.UserId, req.UrlIds)
	return &DeleteUserURLsResponse{
		Error:      result.Error,
		StatusCode: int32(result.Status),
	}, nil
}

// Ping проверяет доступность сервиса
func (s *ShortenerServer) Ping(ctx context.Context, req *PingRequest) (*PingResponse, error) {
	result := s.service.Ping(ctx)
	return &PingResponse{
		Error:      result.Error,
		StatusCode: int32(result.Status),
	}, nil
}

// GetStats получает статистику сервиса
func (s *ShortenerServer) GetStats(ctx context.Context, req *GetStatsRequest) (*GetStatsResponse, error) {
	result := s.service.GetStats(ctx)
	return &GetStatsResponse{
		Urls:       int32(result.URLs),
		Users:      int32(result.Users),
		Error:      result.Error,
		StatusCode: int32(result.Status),
	}, nil
}
