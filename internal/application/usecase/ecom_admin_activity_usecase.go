package usecase

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomAdminActivityService — Sprint 5 Chunk 9 (2 Aug 2026).
// Audit trail per aksi admin. Log dari usecase lain via Log() call.
// Best-effort: failure log warn tapi tidak block aksi utama.
type EcomAdminActivityService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomAdminActivityService(ctx context.Context, db *gorm.DB) *EcomAdminActivityService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomAdminActivityService{DB: db, Log: logger}
}

// Record — insert audit row. Best-effort — kalau gagal log warn, jangan block.
// Meta optional (nil-safe).
func (s *EcomAdminActivityService) Record(adminID, action, target, description string, meta interface{}) {
	var metaJSON *string
	if meta != nil {
		if b, err := json.Marshal(meta); err == nil {
			str := string(b)
			metaJSON = &str
		}
	}
	var targetPtr *string
	if target != "" {
		targetPtr = &target
	}
	entry := entity.EcomActivityLog{
		ID:          uuid.New().String(),
		AdminID:     adminID,
		Action:      action,
		Target:      targetPtr,
		Description: description,
		Meta:        metaJSON,
	}
	if err := s.DB.Create(&entry).Error; err != nil {
		s.Log.Warn().Err(err).Str("action", action).Msg("activity log insert failed (best-effort)")
	}
}

// List — filter opsional: action + admin_id + search di description.
func (s *EcomAdminActivityService) List(action, adminID, search string, limit int) ([]entity.EcomActivityLog, *dto.ApiError) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := s.DB.Model(&entity.EcomActivityLog{})
	if action != "" && action != "all" {
		q = q.Where("action = ?", action)
	}
	if adminID != "" {
		q = q.Where("admin_id = ?", adminID)
	}
	if s := strings.TrimSpace(search); s != "" {
		like := "%" + s + "%"
		q = q.Where("description LIKE ? OR target LIKE ?", like, like)
	}
	var rows []entity.EcomActivityLog
	if err := q.Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal ambil activity log"}
	}
	// Batch fetch nama admin.
	if len(rows) > 0 {
		ids := make(map[string]struct{})
		for _, r := range rows {
			ids[r.AdminID] = struct{}{}
		}
		idList := make([]string, 0, len(ids))
		for id := range ids {
			idList = append(idList, id)
		}
		var users []entity.User
		s.DB.Select("id, fullname").Where("id IN ?", idList).Find(&users)
		names := map[string]string{}
		for _, u := range users {
			names[u.ID] = u.FullName
		}
		for i := range rows {
			rows[i].AdminName = names[rows[i].AdminID]
		}
	}
	return rows, nil
}
