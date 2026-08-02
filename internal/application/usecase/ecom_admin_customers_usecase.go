package usecase

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomAdminCustomersService — Sprint 4 Chunk 1 (30 Jul 2026).
// Manage daftar customer registered (users.role='user') + drill-down riwayat
// order ecom. Scope: HANYA ecom orders (order_source='ecom'), bukan POS.
type EcomAdminCustomersService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomAdminCustomersService(ctx context.Context, db *gorm.DB) *EcomAdminCustomersService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomAdminCustomersService{DB: db, Log: logger}
}

// List — cari + paginate. Search cover nama, email, phone.
// Sort: created_at DESC default (customer baru dulu). Aggregate order stats
// via LEFT JOIN supaya customer belum pernah order tetap muncul.
func (s *EcomAdminCustomersService) List(search string, page, limit int) (*dto.EcomAdminCustomerListResponse, *dto.ApiError) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	base := s.DB.Table("users u").
		Where("u.role = ? AND u.deleted_at IS NULL", "user")

	if search != "" {
		like := "%" + strings.ToLower(search) + "%"
		base = base.Where(
			"LOWER(u.fullname) LIKE ? OR LOWER(u.email) LIKE ? OR u.phone LIKE ?",
			like, like, like,
		)
	}

	// Count total sekali untuk pagination footer.
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		s.Log.Error().Err(err).Msg("Failed to count customers")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal ambil daftar customer"}
	}

	// Query utama dengan LEFT JOIN ke ecom orders. Pakai subquery agregat
	// supaya 1 customer = 1 row (bukan cartesian dengan orders).
	rows := []struct {
		ID            string
		FullName      string
		Email         string
		Phone         string
		IsActive      bool
		OrderCount    int
		TotalSpent    float64
		LastOrderDate *string
		CreatedAt     string
	}{}

	err := base.
		Select(`u.id, u.fullname AS full_name, u.email, u.phone, u.is_active,
			COALESCE(agg.order_count, 0) AS order_count,
			COALESCE(agg.total_spent, 0)  AS total_spent,
			agg.last_order_date,
			DATE_FORMAT(u.created_at, '%Y-%m-%dT%H:%i:%sZ') AS created_at`).
		Joins(`LEFT JOIN (
			SELECT o.ecom_user_id,
			       COUNT(*) AS order_count,
			       SUM(CASE WHEN o.status = 'completed' THEN o.total ELSE 0 END) AS total_spent,
			       DATE_FORMAT(MAX(o.created_at), '%Y-%m-%dT%H:%i:%sZ') AS last_order_date
			FROM orders o
			WHERE o.order_source = 'ecom' AND o.deleted_at IS NULL AND o.ecom_user_id IS NOT NULL
			GROUP BY o.ecom_user_id
		) agg ON agg.ecom_user_id = u.id`).
		Order("u.created_at DESC").
		Limit(limit).Offset(offset).
		Scan(&rows).Error
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to list customers")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal ambil daftar customer"}
	}

	items := make([]dto.EcomAdminCustomerListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, dto.EcomAdminCustomerListItem{
			ID:            r.ID,
			FullName:      r.FullName,
			Email:         r.Email,
			Phone:         r.Phone,
			IsActive:      r.IsActive,
			OrderCount:    r.OrderCount,
			TotalSpent:    r.TotalSpent,
			LastOrderDate: r.LastOrderDate,
			CreatedAt:     r.CreatedAt,
		})
	}

	return &dto.EcomAdminCustomerListResponse{
		Items: items,
		Total: total,
	}, nil
}

// GetDetail — customer + agregat + 20 recent orders. Dipakai di modal detail.
func (s *EcomAdminCustomersService) GetDetail(customerID string) (*dto.EcomAdminCustomerDetail, *dto.ApiError) {
	// Ambil basic user info.
	base := s.DB.Table("users u").Where("u.id = ? AND u.role = 'user' AND u.deleted_at IS NULL", customerID)

	basic := struct {
		ID        string
		FullName  string
		Email     string
		Phone     string
		IsActive  bool
		CreatedAt string
	}{}
	if err := base.
		Select("u.id, u.fullname AS full_name, u.email, u.phone, u.is_active, DATE_FORMAT(u.created_at, '%Y-%m-%dT%H:%i:%sZ') AS created_at").
		Take(&basic).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Customer tidak ditemukan"}
	}

	// Agregat per-status.
	agg := struct {
		OrderCount     int
		TotalSpent     float64
		LastOrderDate  *string
		AvgOrderValue  float64
		CompletedCount int
		PendingCount   int
		CancelledCount int
	}{}
	s.DB.Table("orders o").
		Where("o.order_source = 'ecom' AND o.ecom_user_id = ? AND o.deleted_at IS NULL", customerID).
		Select(`COUNT(*) AS order_count,
			SUM(CASE WHEN o.status = 'completed' THEN o.total ELSE 0 END) AS total_spent,
			DATE_FORMAT(MAX(o.created_at), '%Y-%m-%dT%H:%i:%sZ') AS last_order_date,
			COALESCE(AVG(CASE WHEN o.status = 'completed' THEN o.total ELSE NULL END), 0) AS avg_order_value,
			SUM(CASE WHEN o.status = 'completed' THEN 1 ELSE 0 END) AS completed_count,
			SUM(CASE WHEN o.status IN ('pending','pending_payment','paid','packed','shipped','delivered') THEN 1 ELSE 0 END) AS pending_count,
			SUM(CASE WHEN o.status = 'cancelled' THEN 1 ELSE 0 END) AS cancelled_count`).
		Scan(&agg)

	// Recent orders (limit 20).
	orderRows := []struct {
		ID        string
		CreatedAt string
		Status    string
		Total     float64
		ItemCount int
	}{}
	s.DB.Table("orders o").
		Where("o.order_source = 'ecom' AND o.ecom_user_id = ? AND o.deleted_at IS NULL", customerID).
		Select(`o.id, DATE_FORMAT(o.created_at, '%Y-%m-%dT%H:%i:%sZ') AS created_at,
			COALESCE(o.ecom_status, o.status) AS status, o.total,
			(SELECT COUNT(*) FROM order_items oi WHERE oi.order_id = o.id) AS item_count`).
		Order("o.created_at DESC").
		Limit(20).
		Scan(&orderRows)

	recent := make([]dto.EcomAdminCustomerOrderRow, 0, len(orderRows))
	for _, r := range orderRows {
		recent = append(recent, dto.EcomAdminCustomerOrderRow{
			ID: r.ID, CreatedAt: r.CreatedAt, Status: r.Status,
			Total: r.Total, ItemCount: r.ItemCount,
		})
	}

	// Address count — signal customer sudah verified alamat.
	var addressCount int64
	s.DB.Table("ecom_addresses").
		Where("user_id = ? AND deleted_at IS NULL", customerID).
		Count(&addressCount)

	return &dto.EcomAdminCustomerDetail{
		EcomAdminCustomerListItem: dto.EcomAdminCustomerListItem{
			ID:            basic.ID,
			FullName:      basic.FullName,
			Email:         basic.Email,
			Phone:         basic.Phone,
			IsActive:      basic.IsActive,
			OrderCount:    agg.OrderCount,
			TotalSpent:    agg.TotalSpent,
			LastOrderDate: agg.LastOrderDate,
			CreatedAt:     basic.CreatedAt,
		},
		RecentOrders:   recent,
		AddressCount:   int(addressCount),
		AvgOrderValue:  agg.AvgOrderValue,
		CompletedCount: agg.CompletedCount,
		PendingCount:   agg.PendingCount,
		CancelledCount: agg.CancelledCount,
	}, nil
}

// SetActive — ban / unban customer. Simpan flag is_active saja (tidak
// soft-delete supaya order history tetap dapat di-drill).
func (s *EcomAdminCustomersService) SetActive(customerID string, active bool) *dto.ApiError {
	res := s.DB.Table("users").
		Where("id = ? AND role = 'user' AND deleted_at IS NULL", customerID).
		Update("is_active", active)
	if res.Error != nil {
		s.Log.Error().Err(res.Error).Msg("Failed to update customer active state")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal update status"}
	}
	if res.RowsAffected == 0 {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Customer tidak ditemukan"}
	}
	return nil
}
