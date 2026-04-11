package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type PushService struct {
	Log     *zerolog.Logger
	Configs *config.Config
	Repo    *repository.PushRepository
}

func NewPushService(ctx context.Context, db *gorm.DB) *PushService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &PushService{
		Log:     logger,
		Configs: configs,
		Repo:    repository.NewPushRepository(ctx, db),
	}
}

func (s *PushService) GetVAPIDPublicKey() string {
	return s.Configs.VAPIDPublicKey
}

func (s *PushService) Subscribe(req dto.SubscribePushRequest, userID string) *dto.ApiError {
	// Check if already subscribed
	existing, _ := s.Repo.FindByEndpoint(req.Endpoint)
	if existing != nil {
		return nil // Already subscribed
	}

	sub := &entity.PushSubscription{
		ID:       uuid.New().String(),
		UserID:   userID,
		Endpoint: req.Endpoint,
		P256dh:   req.P256dh,
		Auth:     req.Auth,
	}

	if err := s.Repo.Create(sub); err != nil {
		s.Log.Error().Err(err).Msg("Failed to create push subscription")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to subscribe"}
	}

	return nil
}

func (s *PushService) Unsubscribe(req dto.UnsubscribePushRequest) *dto.ApiError {
	if err := s.Repo.DeleteByEndpoint(req.Endpoint); err != nil {
		s.Log.Error().Err(err).Msg("Failed to delete push subscription")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to unsubscribe"}
	}
	return nil
}

func (s *PushService) SendToAll(title, body, url string) {
	subs, err := s.Repo.FindAll()
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch push subscriptions")
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
		"url":   url,
	})

	for _, sub := range subs {
		s.sendPush(&sub, payload)
	}
}

func (s *PushService) sendPush(sub *entity.PushSubscription, payload []byte) {
	subscription := &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}

	resp, err := webpush.SendNotification(payload, subscription, &webpush.Options{
		Subscriber:      fmt.Sprintf("mailto:admin@%s", s.Configs.AppName),
		VAPIDPublicKey:  s.Configs.VAPIDPublicKey,
		VAPIDPrivateKey: s.Configs.VAPIDPrivateKey,
	})
	if err != nil {
		s.Log.Warn().Err(err).Str("endpoint", sub.Endpoint).Msg("Failed to send push")
		// If subscription expired (410 Gone), delete it
		if resp != nil && resp.StatusCode == 410 {
			s.Repo.DeleteByEndpoint(sub.Endpoint)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 410 {
		s.Repo.DeleteByEndpoint(sub.Endpoint)
	}
}
