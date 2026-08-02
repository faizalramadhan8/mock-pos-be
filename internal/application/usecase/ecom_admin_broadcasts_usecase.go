package usecase

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomAdminBroadcastsService — Sprint 5 Chunk 6 (2 Aug 2026).
// Kompose + fan-out push notif ke ecom customers + record history.
type EcomAdminBroadcastsService struct {
	DB   *gorm.DB
	Log  *zerolog.Logger
	Push *PushService
}

func NewEcomAdminBroadcastsService(ctx context.Context, db *gorm.DB, push *PushService) *EcomAdminBroadcastsService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomAdminBroadcastsService{DB: db, Log: logger, Push: push}
}

// Send — kirim broadcast + record ke ecom_broadcasts. Fan-out synchronous
// (blocking) supaya admin lihat delivered/failed count di response. Skala
// Bu Santi kecil (< 100 sub), no queue perlu.
func (s *EcomAdminBroadcastsService) Send(adminID string, req dto.EcomBroadcastCreateRequest) (*dto.EcomBroadcastResponse, *dto.ApiError) {
	// Fan out.
	var urlPtr *string
	if req.URL != "" {
		u := req.URL
		urlPtr = &u
	}
	result := s.Push.SendToEcomCustomers(req.Title, req.Body, req.URL)

	broadcast := entity.EcomBroadcast{
		ID:               uuid.New().String(),
		Title:            req.Title,
		Body:             req.Body,
		URL:              urlPtr,
		DeliveredCount:   result.Delivered,
		FailedCount:      result.Failed,
		TotalSubscribers: result.TotalSubscribers,
		SentBy:           adminID,
		SentAt:           time.Now(),
	}
	if err := s.DB.Create(&broadcast).Error; err != nil {
		s.Log.Error().Err(err).Msg("Failed to record broadcast history")
		// Broadcast tetap sent — tapi record gagal. Return partial success.
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Notif terkirim tapi history gagal disimpan",
		}
	}
	return s.toResponse(&broadcast, ""), nil
}

// List — history 100 broadcast terakhir + nama admin sender.
func (s *EcomAdminBroadcastsService) List() ([]dto.EcomBroadcastResponse, *dto.ApiError) {
	var rows []entity.EcomBroadcast
	if err := s.DB.Order("sent_at DESC").Limit(100).Find(&rows).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal ambil history"}
	}
	// Batch fetch nama admin.
	names := map[string]string{}
	if len(rows) > 0 {
		ids := make([]string, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.SentBy)
		}
		var users []entity.User
		s.DB.Select("id, fullname").Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			names[u.ID] = u.FullName
		}
	}
	out := make([]dto.EcomBroadcastResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *s.toResponse(&rows[i], names[rows[i].SentBy]))
	}
	return out, nil
}

func (s *EcomAdminBroadcastsService) toResponse(b *entity.EcomBroadcast, senderName string) *dto.EcomBroadcastResponse {
	url := ""
	if b.URL != nil {
		url = *b.URL
	}
	return &dto.EcomBroadcastResponse{
		ID:               b.ID,
		Title:            b.Title,
		Body:             b.Body,
		URL:              url,
		DeliveredCount:   b.DeliveredCount,
		FailedCount:      b.FailedCount,
		TotalSubscribers: b.TotalSubscribers,
		SentBy:           b.SentBy,
		SentByName:       senderName,
		SentAt:           b.SentAt.Format(time.RFC3339),
	}
}
