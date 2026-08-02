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

// SendToUser — Sprint 2 #5. Push targeted ke 1 user (ecom customer). Loop
// semua subscription user tersebut (customer bisa install PWA di > 1 device).
// Best-effort — kalau ada sub expired, auto-delete di sendPush.
func (s *PushService) SendToUser(userID, title, body, url string) {
	if userID == "" || s.Configs.VAPIDPublicKey == "" {
		return
	}
	subs, err := s.Repo.FindByUserID(userID)
	if err != nil {
		s.Log.Warn().Err(err).Str("user_id", userID).Msg("push: fetch subs failed")
		return
	}
	if len(subs) == 0 {
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

// sendPush — return true kalau delivered (2xx). Sprint 5 (2 Aug 2026):
// signature diubah dari void ke bool supaya caller (broadcast) bisa hitung
// delivered vs failed. Existing callers yang ignore return tetap works.
func (s *PushService) sendPush(sub *entity.PushSubscription, payload []byte) bool {
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
		if resp != nil && resp.StatusCode == 410 {
			s.Repo.DeleteByEndpoint(sub.Endpoint)
		}
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 410 {
		s.Repo.DeleteByEndpoint(sub.Endpoint)
		return false
	}
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// BroadcastResult — Sprint 5 Chunk 6. Metric hasil broadcast, buat direkam
// ke ecom_broadcasts + ditampilkan di history.
type BroadcastResult struct {
	Delivered        int
	Failed           int
	TotalSubscribers int
}

// SendToEcomCustomers — Sprint 5 Chunk 6 (2 Aug 2026). Fan-out push ke SEMUA
// customer ecom (users.role='user'). Exclude admin subs supaya broadcast
// promo tidak muncul di tab POS admin.
func (s *PushService) SendToEcomCustomers(title, body, url string) BroadcastResult {
	res := BroadcastResult{}
	if s.Configs.VAPIDPublicKey == "" {
		return res
	}
	// JOIN users role='user' filter — cegah broadcast reach kasir/admin.
	var subs []entity.PushSubscription
	if err := s.Repo.DB.
		Joins("JOIN users u ON u.id = push_subscriptions.user_id").
		Where("u.role = ? AND u.deleted_at IS NULL AND u.is_active = 1", "user").
		Find(&subs).Error; err != nil {
		s.Log.Error().Err(err).Msg("broadcast: fetch ecom subs failed")
		return res
	}
	res.TotalSubscribers = len(subs)
	if len(subs) == 0 {
		return res
	}
	payload, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
		"url":   url,
	})
	for i := range subs {
		if s.sendPush(&subs[i], payload) {
			res.Delivered++
		} else {
			res.Failed++
		}
	}
	return res
}
